package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// Public report links are explicit, bearer-capability grants, not organization logins.
func (s *Server) shareInspection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	inspectionID := r.PathValue("inspection")
	if !validID(inspectionID) {
		fail(w, 400, "invalid inspection ID")
		return
	}
	var exists bool
	err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM inspections
		WHERE organization_id=$1 AND id=$2 AND deleted_at IS NULL)`, r.PathValue("org"), inspectionID).Scan(&exists)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if !exists {
		fail(w, 404, "inspection not found")
		return
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		fail(w, 500, "unavailable")
		return
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().Add(7 * 24 * time.Hour).UTC()
	var id string
	err = s.db.QueryRow(r.Context(), `INSERT INTO report_links
		(organization_id,inspection_id,created_by,token_hash,expires_at)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		r.PathValue("org"), inspectionID, who(r).UserID, hash[:], expires).Scan(&id)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 201, map[string]any{"id": id, "token": token, "expires_at": expires})
}

func (s *Server) listReportLinks(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	id := r.PathValue("inspection")
	if !validID(id) {
		fail(w, 400, "invalid inspection ID")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,expires_at,revoked_at,created_at
		FROM report_links WHERE organization_id=$1 AND inspection_id=$2
		ORDER BY created_at DESC LIMIT 100`, r.PathValue("org"), id)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id string
		var expires, created time.Time
		var revoked *time.Time
		if err := rows.Scan(&id, &expires, &revoked, &created); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "expires_at": expires, "revoked_at": revoked, "created_at": created})
	}
	if rows.Err() != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}

func (s *Server) revokeReportLink(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	linkID := r.PathValue("link")
	if !validID(linkID) {
		fail(w, 400, "invalid link ID")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE report_links SET revoked_at=now()
		WHERE id=$1 AND organization_id=$2 AND revoked_at IS NULL`, linkID, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "active link not found")
		return
	}
	write(w, 200, map[string]string{"status": "revoked"})
}

func (s *Server) publicReport(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if len(token) != 43 {
		fail(w, 404, "report link not found")
		return
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		fail(w, 404, "report link not found")
		return
	}
	hash := sha256.Sum256([]byte(token))
	var inspectionID, propertyName, kind, status string
	var date time.Time
	err = s.db.QueryRow(r.Context(), `SELECT i.id,p.name,i.inspection_type,i.status,i.created_at
		FROM report_links l JOIN inspections i ON i.organization_id=l.organization_id AND i.id=l.inspection_id
		JOIN properties p ON p.organization_id=i.organization_id AND p.id=i.property_id
		WHERE l.token_hash=$1 AND l.revoked_at IS NULL AND l.expires_at>now() AND i.deleted_at IS NULL AND p.deleted_at IS NULL`, hash[:]).
		Scan(&inspectionID, &propertyName, &kind, &status, &date)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "report link not found or expired")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT title,notes,severity,status FROM inspection_items
		WHERE inspection_id=$1 AND deleted_at IS NULL ORDER BY created_at LIMIT 500`, inspectionID)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]string, 0)
	for rows.Next() {
		var title, notes, severity, state string
		if err := rows.Scan(&title, &notes, &severity, &state); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		items = append(items, map[string]string{"title": title, "notes": notes, "severity": severity, "status": state})
	}
	if rows.Err() != nil {
		fail(w, 503, "unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	write(w, 200, map[string]any{"inspection_type": kind, "status": status,
		"property_name": propertyName, "created_at": date, "items": items})
}
