package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func hashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func validInviteToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
func (s *Server) inviteMember(w http.ResponseWriter, r *http.Request) {
	caller, ok := s.role(r, "owner", "manager")
	if !ok {
		fail(w, 403, "forbidden")
		return
	}
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	email := normalizedEmail(in.Email)
	if email == "" || (in.Role != "manager" && in.Role != "inspector") {
		fail(w, 400, "invalid invitation")
		return
	}
	if in.Role == "manager" && caller != "owner" {
		fail(w, 403, "only owner can invite a manager")
		return
	}
	var already bool
	err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM organization_members m JOIN users u ON u.id=m.user_id
 WHERE m.organization_id=$1 AND lower(u.email)=$2 AND u.email_verified_at IS NOT NULL)`, r.PathValue("org"), email).Scan(&already)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if already {
		fail(w, 409, "email already belongs to a member")
		return
	}
	token, digest, err := generateInvitationToken()
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	// Serialize invitation issuance within the organization, including resends.
	_, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	var dailyCount, hourlyRecipientCount int
	err = tx.QueryRow(r.Context(), `SELECT count(*),count(*) FILTER (WHERE email=$2 AND created_at>now()-interval '1 hour')
		FROM organization_invitations WHERE organization_id=$1 AND created_at>now()-interval '24 hours'`,
		r.PathValue("org"), email).Scan(&dailyCount,&hourlyRecipientCount)
	if err != nil {
		fail(w,503,"unavailable")
		return
	}
	if dailyCount>=30 || hourlyRecipientCount>=3 {
		fail(w,429,"invitation rate limited")
		return
	}
	// A resend supersedes older links rather than leaving multiple valid capabilities.
	_, err = tx.Exec(r.Context(), `UPDATE organization_invitations SET revoked_at=now()
		WHERE organization_id=$1 AND email=$2 AND revoked_at IS NULL AND accepted_at IS NULL`,
		r.PathValue("org"),email)
	if err != nil {
		fail(w,503,"unavailable")
		return
	}
	var id, orgName string
	var expires time.Time
	err = tx.QueryRow(r.Context(), `INSERT INTO organization_invitations(organization_id,email,role,token_hash,created_by,expires_at)
 VALUES($1,$2,$3,$4,$5,now()+interval '7 days')
 RETURNING id,expires_at`, r.PathValue("org"), email, in.Role, digest, who(r).UserID).Scan(&id, &expires)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	err = tx.QueryRow(r.Context(), `SELECT name FROM organizations WHERE id=$1`, r.PathValue("org")).Scan(&orgName)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	origin := strings.TrimRight(os.Getenv("PUBLIC_WEB_URL"), "/")
	provider := os.Getenv("RESEND_API_KEY") != "" && os.Getenv("EMAIL_FROM") != ""
	if provider {
		if origin == "" || !strings.HasPrefix(origin, "https://") {
			fail(w, 503, "PUBLIC_WEB_URL must be configured with HTTPS")
			return
		}
		link := origin + "/invite/" + token
		message := "<p>You have been invited to join <strong>" + html.EscapeString(orgName) + "</strong> as " + html.EscapeString(in.Role) + ".</p><p><a href=\"" + html.EscapeString(link) + "\">Accept your invitation</a></p><p>Expires in 7 days.</p>"
		if err := sendTransactionalEmail(r.Context(), email, "Join "+orgName+" on Inspectly", message); err != nil {
			s.logger.Error("invite delivery failed", "error", err)
			fail(w, 503, "invitation email delivery unavailable")
			return
		}
	} else if s.env != "development" {
		fail(w, 503, "email provider not configured")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	// Raw token is returned only once, to an authorized organization administrator.
	write(w, 201, map[string]any{"id": id, "role": in.Role, "email": email, "token": token, "expires_at": expires})
}
func (s *Server) listInvitations(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,email,role,created_at,expires_at,revoked_at,accepted_at
 FROM organization_invitations WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 100`, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, email, role string
		var created, expires time.Time
		var revoked, accepted *time.Time
		if err = rows.Scan(&id, &email, &role, &created, &expires, &revoked, &accepted); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "email": email, "role": role, "created_at": created, "expires_at": expires, "revoked_at": revoked, "accepted_at": accepted})
	}
	if rows.Err() != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}
func (s *Server) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	caller, ok := s.role(r, "owner", "manager")
	if !ok {
		fail(w, 403, "forbidden")
		return
	}
	id := r.PathValue("invite")
	if !validID(id) {
		fail(w, 400, "invalid invitation")
		return
	}
	var role string
	err := s.db.QueryRow(r.Context(), `SELECT role FROM organization_invitations WHERE id=$1 AND organization_id=$2`, id, r.PathValue("org")).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "invitation not found")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if role == "manager" && caller != "owner" {
		fail(w, 403, "only owner can manage a manager invite")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE organization_invitations SET revoked_at=now()
 WHERE id=$1 AND organization_id=$2 AND revoked_at IS NULL AND accepted_at IS NULL`, id, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "invitation already used or revoked")
		return
	}
	write(w, 200, map[string]string{"status": "revoked"})
}
func (s *Server) invitationPreview(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !validInviteToken(token) {
		fail(w, 404, "invitation not found")
		return
	}
	var name, role string
	var expires time.Time
	err := s.db.QueryRow(r.Context(), `SELECT o.name,i.role,i.expires_at FROM organization_invitations i
 JOIN organizations o ON o.id=i.organization_id
 WHERE i.token_hash=$1 AND i.revoked_at IS NULL AND i.accepted_at IS NULL AND i.expires_at>now()`, hashInvitationToken(token)).Scan(&name, &role, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "invitation not found")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]any{"organization_name": name, "role": role, "expires_at": expires})
}
func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !validInviteToken(token) {
		fail(w, 404, "invitation not found")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var org, email, role string
	err = tx.QueryRow(r.Context(), `SELECT organization_id,email,role FROM organization_invitations
 WHERE token_hash=$1 AND revoked_at IS NULL AND accepted_at IS NULL AND expires_at>now()
 FOR UPDATE`, hashInvitationToken(token)).Scan(&org, &email, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 409, "invitation expired or already accepted")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	var userEmail *string
	var verified *time.Time
	err = tx.QueryRow(r.Context(), `SELECT email,email_verified_at FROM users WHERE id=$1`, who(r).UserID).Scan(&userEmail, &verified)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if userEmail == nil || verified == nil || strings.ToLower(*userEmail) != email {
		fail(w, 403, "verify the invited email address first")
		return
	}
	tag, err := tx.Exec(r.Context(), `INSERT INTO organization_members(organization_id,user_id,role)
 VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, org, who(r).UserID, role)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "already an organization member")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE organization_invitations SET accepted_at=now(),accepted_by=$2
 WHERE token_hash=$1`, hashInvitationToken(token), who(r).UserID)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "accepted", "organization_id": org, "role": role})
}
