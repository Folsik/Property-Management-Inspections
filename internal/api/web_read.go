package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type inspectionSummary struct {
	ID           string    `json:"id"`
	PropertyID   string    `json:"property_id"`
	PropertyName string    `json:"property_name"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	OpenItems    int       `json:"open_items"`
	TotalItems   int       `json:"total_items"`
}

func (s *Server) listInspections(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, http.StatusForbidden, "forbidden")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT i.id, i.property_id, p.name, i.inspection_type,
		i.status, i.created_at,
		(SELECT count(*) FROM inspection_items x WHERE x.inspection_id=i.id AND x.status!='resolved'),
		(SELECT count(*) FROM inspection_items x WHERE x.inspection_id=i.id)
		FROM inspections i JOIN properties p ON p.organization_id=i.organization_id AND p.id=i.property_id
		WHERE i.organization_id=$1 ORDER BY i.created_at DESC LIMIT 200`, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer rows.Close()
	items := make([]inspectionSummary, 0)
	for rows.Next() {
		var in inspectionSummary
		if err := rows.Scan(&in.ID, &in.PropertyID, &in.PropertyName, &in.Type, &in.Status,
			&in.CreatedAt, &in.OpenItems, &in.TotalItems); err != nil {
			fail(w, 503, "database unavailable")
			return
		}
		items = append(items, in)
	}
	if rows.Err() != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}

func (s *Server) inspectionDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	id := r.PathValue("inspection")
	if !validID(id) {
		fail(w, 400, "invalid inspection ID")
		return
	}
	var in inspectionSummary
	err := s.db.QueryRow(r.Context(), `SELECT i.id, i.property_id, p.name, i.inspection_type,
		i.status, i.created_at,
		(SELECT count(*) FROM inspection_items x WHERE x.inspection_id=i.id AND x.status!='resolved'),
		(SELECT count(*) FROM inspection_items x WHERE x.inspection_id=i.id)
		FROM inspections i JOIN properties p ON p.organization_id=i.organization_id AND p.id=i.property_id
		WHERE i.organization_id=$1 AND i.id=$2`, r.PathValue("org"), id).
		Scan(&in.ID, &in.PropertyID, &in.PropertyName, &in.Type, &in.Status, &in.CreatedAt, &in.OpenItems, &in.TotalItems)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "inspection not found")
		return
	}
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, in)
}

func (s *Server) updateInspection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	id := r.PathValue("inspection")
	if !validID(id) {
		fail(w, 400, "invalid inspection ID")
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Status != "in_progress" && in.Status != "completed" && in.Status != "cancelled" {
		fail(w, 400, "invalid status")
		return
	}
	var status string
	err := s.db.QueryRow(r.Context(), `UPDATE inspections SET status=$3
		WHERE organization_id=$1 AND id=$2 AND (
			(status='draft' AND $3 IN ('in_progress','cancelled')) OR
			(status='in_progress' AND $3 IN ('completed','cancelled'))
		) RETURNING status`, r.PathValue("org"), id, in.Status).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 409, "inspection not found or transition not allowed")
		return
	}
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]string{"id": id, "status": status})
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	id := r.PathValue("inspection")
	if !validID(id) {
		fail(w, 400, "invalid inspection ID")
		return
	}
	var present bool
	if err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM inspections
		WHERE organization_id=$1 AND id=$2)`, r.PathValue("org"), id).Scan(&present); err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	if !present {
		fail(w, 404, "inspection not found")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,client_id,title,notes,severity,status,created_at
		FROM inspection_items WHERE organization_id=$1 AND inspection_id=$2
		ORDER BY created_at DESC LIMIT 500`, r.PathValue("org"), id)
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, clientID, title, notes, severity, status string
		var created time.Time
		if err := rows.Scan(&id, &clientID, &title, &notes, &severity, &status, &created); err != nil {
			fail(w, 503, "database unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "client_id": clientID, "title": title,
			"notes": notes, "severity": severity, "status": status, "created_at": created})
	}
	if rows.Err() != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	id, itemID := r.PathValue("inspection"), r.PathValue("item")
	if !validID(id) || !validID(itemID) {
		fail(w, 400, "invalid ID")
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Status != "open" && in.Status != "in_progress" && in.Status != "resolved" {
		fail(w, 400, "invalid status")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE inspection_items SET status=$4
		WHERE organization_id=$1 AND inspection_id=$2 AND id=$3
		AND EXISTS(SELECT 1 FROM inspections WHERE organization_id=$1 AND id=$2
			AND status IN ('draft','in_progress'))`,
		r.PathValue("org"), id, itemID, in.Status)
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "item not found or inspection closed")
		return
	}
	write(w, 200, map[string]string{"id": itemID, "status": in.Status})
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT u.id,u.phone,u.full_name,m.role,m.created_at
		FROM organization_members m JOIN users u ON u.id=m.user_id
		WHERE m.organization_id=$1 ORDER BY m.created_at LIMIT 200`, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, phone, name, role string
		var joined time.Time
		if err := rows.Scan(&id, &phone, &name, &role, &joined); err != nil {
			fail(w, 503, "database unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "phone": phone, "full_name": name, "role": role, "joined_at": joined})
	}
	if rows.Err() != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}

func (s *Server) listMedia(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	inspection := r.PathValue("inspection")
	if !validID(inspection) {
		fail(w, 400, "invalid inspection ID")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,mime_type,size_bytes,state,created_at
		FROM media_attachments WHERE organization_id=$1 AND inspection_id=$2 AND state<>'deleted'
		ORDER BY created_at DESC LIMIT 200`, r.PathValue("org"), inspection)
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, mime, state string
		var size int64
		var created time.Time
		if err := rows.Scan(&id, &mime, &size, &state, &created); err != nil {
			fail(w, 503, "database unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "mime_type": mime, "size_bytes": size, "state": state, "created_at": created})
	}
	if rows.Err() != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}

// inspectable job state is informational; no PDF or AI report is claimed to exist until workers complete.
func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,kind,state,attempts,created_at
		FROM jobs WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 100`, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, kind, state string
		var attempts int
		var created time.Time
		if err := rows.Scan(&id, &kind, &state, &attempts, &created); err != nil {
			fail(w, 503, "database unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "kind": kind, "state": state, "attempts": attempts, "created_at": created})
	}
	if rows.Err() != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FullName string `json:"full_name"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.FullName = strings.TrimSpace(in.FullName)
	if len(in.FullName) > 120 {
		fail(w, 400, "name too long")
		return
	}
	var name string
	err := s.db.QueryRow(r.Context(), `UPDATE users SET full_name=$2, updated_at=now() WHERE id=$1
		RETURNING full_name`, who(r).UserID, in.FullName).Scan(&name)
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]string{"full_name": name})
}
