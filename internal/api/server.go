package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Folsik/Property-Management-Inspections/internal/auth"
)

type Server struct {
	db                *pgxpool.Pool
	tokens            *auth.Manager
	logger            *slog.Logger
	env               string
	storageHTTPClient *http.Client
	trustedBFFSecret  string
}

func New(db *pgxpool.Pool, tokens *auth.Manager, logger *slog.Logger, env string) *Server {
	return &Server{db: db, tokens: tokens, logger: logger, env: env, storageHTTPClient: &http.Client{Timeout: 10 * time.Second}}
}

type key string

const principal key = "principal"

var uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")
var phonePattern = regexp.MustCompile("^[+][1-9][0-9]{7,14}$")

func validID(id string) bool { return uuidPattern.MatchString(id) }
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", s.health)
	m.HandleFunc("GET /v1/public/reports/{token}", s.publicReport)
	m.HandleFunc("GET /v1/invitations/{token}", s.invitationPreview)
	m.HandleFunc("POST /v1/auth/otp/request", s.limitAuth("otp-request", 60, 10*time.Minute, s.requestOTP))
	m.HandleFunc("POST /v1/auth/otp/verify", s.limitAuth("otp-verify", 120, 10*time.Minute, s.verifyOTP))
	m.HandleFunc("POST /v1/auth/pin/login", s.limitAuth("legacy-pin-login", 120, 10*time.Minute, s.pinLogin))
	m.HandleFunc("POST /v1/auth/password/login", s.limitAuth("password-login", 150, 10*time.Minute, s.passwordLogin))
	m.HandleFunc("POST /v1/auth/refresh", s.limitAuth("refresh", 300, 10*time.Minute, s.refreshAccess))
	m.HandleFunc("POST /v1/auth/refresh/logout", s.logoutRefresh)
	m.HandleFunc("POST /v1/auth/recovery/request", s.limitAuth("recovery-request", 60, 10*time.Minute, s.requestPasswordRecovery))
	m.HandleFunc("POST /v1/auth/recovery/complete", s.limitAuth("recovery-complete", 100, 10*time.Minute, s.completePasswordRecovery))
	m.Handle("POST /v1/auth/password", s.protect(http.HandlerFunc(s.setPassword)))
	m.Handle("POST /v1/auth/password/change", s.protect(http.HandlerFunc(s.changePassword)))
	m.Handle("POST /v1/auth/pin", s.protect(http.HandlerFunc(s.setPIN)))
	m.Handle("POST /v1/auth/logout", s.protect(http.HandlerFunc(s.logout)))
	m.Handle("GET /v1/me", s.protect(http.HandlerFunc(s.me)))
	m.Handle("GET /v1/auth/sessions", s.protect(http.HandlerFunc(s.listAuthSessions)))
	m.Handle("DELETE /v1/auth/sessions/{session}", s.protect(http.HandlerFunc(s.revokeAuthSession)))
	m.Handle("GET /v1/onboarding", s.protect(http.HandlerFunc(s.onboardingStatus)))
	m.Handle("POST /v1/onboarding/complete", s.protect(http.HandlerFunc(s.completeOnboarding)))
	m.Handle("POST /v1/email/request", s.protect(s.limitAuth("email-request", 60, 10*time.Minute, s.requestEmailOTP)))
	m.Handle("POST /v1/email/verify", s.protect(s.limitAuth("email-verify", 120, 10*time.Minute, s.verifyEmailOTP)))
	m.Handle("POST /v1/invitations/{token}/accept", s.protect(http.HandlerFunc(s.acceptInvitation)))
	m.Handle("POST /v1/organizations", s.protect(http.HandlerFunc(s.createOrganization)))
	m.Handle("GET /v1/organizations", s.protect(http.HandlerFunc(s.listOrganizations)))
	m.Handle("GET /v1/organizations/{org}/members", s.protect(http.HandlerFunc(s.listMembers)))
	m.Handle("GET /v1/organizations/{org}/jobs", s.protect(http.HandlerFunc(s.listJobs)))
	m.Handle("GET /v1/organizations/{org}/jobs/metrics", s.protect(http.HandlerFunc(s.jobMetrics)))
	m.Handle("POST /v1/organizations/{org}/inspections/{inspection}/transcriptions", s.protect(http.HandlerFunc(s.enqueueTranscription)))
	m.Handle("POST /v1/organizations/{org}/jobs/{job}/retry", s.protect(http.HandlerFunc(s.requeueFailedJob)))
	m.Handle("GET /v1/organizations/{org}/inspections/{inspection}/reports/{media}/pdf", s.protect(http.HandlerFunc(s.downloadGeneratedReport)))
	m.Handle("PATCH /v1/me", s.protect(http.HandlerFunc(s.updateProfile)))
	m.Handle("GET /v1/organizations/{org}/contacts", s.protect(http.HandlerFunc(s.listContacts)))
	m.Handle("POST /v1/organizations/{org}/contacts", s.protect(http.HandlerFunc(s.createContact)))
	m.HandleFunc("POST /v1/organizations/{org}/members", func(w http.ResponseWriter, r *http.Request) {
		fail(w, 410, "Direct member addition is disabled; send an email invitation")
	})
	m.Handle("GET /v1/organizations/{org}/invitations", s.protect(http.HandlerFunc(s.listInvitations)))
	m.Handle("POST /v1/organizations/{org}/invitations", s.protect(http.HandlerFunc(s.inviteMember)))
	m.Handle("DELETE /v1/organizations/{org}/invitations/{invite}", s.protect(http.HandlerFunc(s.revokeInvitation)))
	m.Handle("POST /v1/organizations/{org}/properties", s.protect(http.HandlerFunc(s.createProperty)))
	m.Handle("GET /v1/organizations/{org}/properties", s.protect(http.HandlerFunc(s.listProperties)))
	m.Handle("GET /v1/organizations/{org}/inspections", s.protect(http.HandlerFunc(s.listInspections)))
	m.Handle("POST /v1/organizations/{org}/inspections/{inspection}/share", s.protect(http.HandlerFunc(s.shareInspection)))
	m.Handle("GET /v1/organizations/{org}/inspections/{inspection}/shares", s.protect(http.HandlerFunc(s.listReportLinks)))
	m.Handle("DELETE /v1/organizations/{org}/shares/{link}", s.protect(http.HandlerFunc(s.revokeReportLink)))
	m.Handle("GET /v1/organizations/{org}/inspections/{inspection}", s.protect(http.HandlerFunc(s.inspectionDetail)))
	m.Handle("PATCH /v1/organizations/{org}/inspections/{inspection}", s.protect(http.HandlerFunc(s.updateInspection)))
	m.Handle("GET /v1/organizations/{org}/inspections/{inspection}/items", s.protect(http.HandlerFunc(s.listItems)))
	m.Handle("PATCH /v1/organizations/{org}/inspections/{inspection}/items/{item}", s.protect(http.HandlerFunc(s.updateItem)))
	m.Handle("GET /v1/organizations/{org}/inspections/{inspection}/media", s.protect(http.HandlerFunc(s.listMedia)))
	m.Handle("POST /v1/organizations/{org}/inspections", s.protect(http.HandlerFunc(s.createInspection)))
	m.Handle("POST /v1/organizations/{org}/inspections/{inspection}/items", s.protect(http.HandlerFunc(s.addItem)))
	m.Handle("DELETE /v1/organizations/{org}/inspections/{inspection}/items/{item}", s.protect(http.HandlerFunc(s.deleteItem)))
	m.Handle("GET /v1/organizations/{org}/sync/pull", s.protect(http.HandlerFunc(s.pullSync)))
	m.Handle("POST /v1/organizations/{org}/sync/push", s.protect(http.HandlerFunc(s.pushSync)))
	m.Handle("POST /v1/organizations/{org}/inspections/{inspection}/media/presign", s.protect(http.HandlerFunc(s.presignMedia)))
	m.Handle("POST /v1/organizations/{org}/inspections/{inspection}/media/{media}/confirm", s.protect(http.HandlerFunc(s.confirmMedia)))
	m.Handle("GET /v1/organizations/{org}/inspections/{inspection}/media/{media}/download", s.protect(http.HandlerFunc(s.downloadMedia)))
	m.Handle("DELETE /v1/organizations/{org}/inspections/{inspection}/media/{media}", s.protect(http.HandlerFunc(s.deleteMedia)))
	return requestID(recoverer(s.logger)(securityHeaders(m)))
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "invalid JSON request")
		return false
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		fail(w, 400, "multiple JSON values or invalid trailing data")
		return false
	}
	return true
}
func bearer(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

type identity struct {
	UserID string
	Claims auth.Claims
}

func who(r *http.Request) identity { p, _ := r.Context().Value(principal).(identity); return p }
func (s *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := s.tokens.Verify(bearer(r))
		if err != nil {
			fail(w, 401, "unauthorized")
			return
		}
		var revoked bool
		err = s.db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM revoked_tokens WHERE jti=$1 AND expires_at>now())", c.TokenID).Scan(&revoked)
		if err != nil {
			s.logger.Error("revocation lookup", "error", err)
			fail(w, 503, "auth unavailable")
			return
		}
		if revoked {
			fail(w, 401, "revoked")
			return
		}
		var epoch int64
		if err := s.db.QueryRow(r.Context(), `SELECT auth_epoch FROM users WHERE id=$1`, c.Subject).Scan(&epoch); err != nil || epoch != c.AuthEpoch {
			fail(w, 401, "session expired")
			return
		}
		if c.SessionID != "" {
			var valid bool
			err = s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM auth_sessions
			WHERE id=$1 AND user_id=$2 AND auth_epoch=$3 AND generation=$4
			AND revoked_at IS NULL AND expires_at>now())`, c.SessionID, c.Subject, c.AuthEpoch, c.SessionGeneration).Scan(&valid)
			if err != nil {
				fail(w, 503, "auth unavailable")
				return
			}
			if !valid {
				fail(w, 401, "session revoked or rotated")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principal, identity{c.Subject, c})))
	})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) requestOTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !phonePattern.MatchString(in.Phone) {
		fail(w, 400, "phone must be E.164")
		return
	}
	if !s.limitIdentifier(w, r, "otp-request", in.Phone, 3, 15*time.Minute) {
		return
	}
	var count int
	err := s.db.QueryRow(r.Context(), "SELECT count(*) FROM otp_challenges WHERE phone=$1 AND created_at>now()-interval '15 minutes'", in.Phone).Scan(&count)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if count >= 3 {
		fail(w, 429, "rate limited")
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		fail(w, 500, "unavailable")
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "unavailable")
		return
	}
	twilio := twilioConfigFromEnv()
	if !twilio.configured() && s.env != "development" {
		fail(w, 503, "OTP provider not configured")
		return
	}
	// Keep challenge insertion atomic with provider delivery: unsuccessful sends never
	// produce a usable OTP record. Production responses never expose the secret code.
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), "INSERT INTO otp_challenges(phone,code_hash,expires_at) VALUES($1,$2,now()+interval '5 minutes')", in.Phone, string(hash))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if twilio.configured() {
		if err := sendTwilioSMS(r.Context(), twilio, in.Phone, "Your Inspectly verification code is "+code+". It expires in 5 minutes."); err != nil {
			s.logger.Error("OTP provider delivery failed", "error", err)
			fail(w, 503, "OTP delivery unavailable")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if !twilio.configured() {
		// Only available when Go runs in development and the SMS provider is unset.
		// Never display this secret through the browser BFF.
		write(w, 202, map[string]string{"status": "local_demo_only", "dev_code": code})
		return
	}
	write(w, 202, map[string]string{"status": "sent"})
}
func (s *Server) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !phonePattern.MatchString(in.Phone) || len(in.Code) != 6 {
		fail(w, 400, "invalid phone or code")
		return
	}
	if !s.limitIdentifier(w, r, "otp-verify", in.Phone, 20, 10*time.Minute) {
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var id, hash string
	var attempts int
	err = tx.QueryRow(r.Context(), "SELECT id,code_hash,attempts FROM otp_challenges WHERE phone=$1 AND consumed_at IS NULL AND expires_at>now() ORDER BY created_at DESC LIMIT 1 FOR UPDATE", in.Phone).Scan(&id, &hash, &attempts)
	if err != nil {
		fail(w, 401, "invalid or expired OTP")
		return
	}
	if attempts >= 5 {
		fail(w, 429, "OTP locked")
		return
	}
	good := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Code)) == nil
	_, err = tx.Exec(r.Context(), "UPDATE otp_challenges SET attempts=attempts+1,consumed_at=CASE WHEN $2 THEN now() ELSE consumed_at END WHERE id=$1", id, good)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if !good {
		_ = tx.Commit(r.Context())
		fail(w, 401, "invalid or expired OTP")
		return
	}
	var user string
	var enrolled bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE phone=$1 AND password_hash IS NOT NULL)`, in.Phone).Scan(&enrolled)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if enrolled {
		fail(w, 409, "This account uses password sign-in")
		return
	}
	err = tx.QueryRow(r.Context(), "INSERT INTO users(phone) VALUES($1) ON CONFLICT(phone) DO UPDATE SET updated_at=users.updated_at RETURNING id", in.Phone).Scan(&user)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	s.issue(w, r, user)
}
func (s *Server) setPIN(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PIN string `json:"pin"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !pinOK(in.PIN) {
		fail(w, 400, "PIN must have 6 non-identical, non-sequential digits")
		return
	}
	h, err := bcrypt.GenerateFromPassword([]byte(in.PIN), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "unavailable")
		return
	}
	tag, err := s.db.Exec(r.Context(), "UPDATE users SET pin_hash=$2,failed_pin_attempts=0,pin_locked_until=NULL,updated_at=now() WHERE id=$1 AND pin_hash IS NULL", who(r).UserID, string(h))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "PIN already set; reauthentication required to change")
		return
	}
	write(w, 200, map[string]string{"status": "pin_set"})
}
func pinOK(pin string) bool {
	if len(pin) != 6 {
		return false
	}
	asc, desc, same := true, true, true
	for i, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
		if i > 0 {
			if pin[i] != pin[0] {
				same = false
			}
			if pin[i] != pin[i-1]+1 {
				asc = false
			}
			if pin[i]+1 != pin[i-1] {
				desc = false
			}
		}
	}
	return !same && !asc && !desc
}
func (s *Server) pinLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		PIN   string `json:"pin"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !phonePattern.MatchString(in.Phone) || len(in.PIN) != 6 {
		fail(w, 401, "invalid credentials")
		return
	}
	if !s.limitIdentifier(w, r, "legacy-pin-login", in.Phone, 20, 10*time.Minute) {
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var id, hash string
	var attempts int
	var locked *time.Time
	err = tx.QueryRow(r.Context(), "SELECT id,coalesce(pin_hash,''),failed_pin_attempts,pin_locked_until FROM users WHERE phone=$1 FOR UPDATE", in.Phone).Scan(&id, &hash, &attempts, &locked)
	if err != nil {
		fail(w, 401, "invalid credentials")
		return
	}
	if locked != nil && locked.After(time.Now()) {
		fail(w, 429, "PIN temporarily locked")
		return
	}
	var passwordSet bool
	if err = tx.QueryRow(r.Context(), `SELECT password_hash IS NOT NULL FROM users WHERE id=$1`, id).Scan(&passwordSet); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if passwordSet {
		fail(w, 401, "use password sign-in")
		return
	}
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.PIN)) != nil {
		_, err = tx.Exec(r.Context(), "UPDATE users SET failed_pin_attempts=CASE WHEN failed_pin_attempts>=4 THEN 0 ELSE failed_pin_attempts+1 END,pin_locked_until=CASE WHEN failed_pin_attempts>=4 THEN now()+interval '15 minutes' ELSE NULL END WHERE id=$1", id)
		if err != nil {
			fail(w, 503, "unavailable")
			return
		}
		_ = tx.Commit(r.Context())
		fail(w, 401, "invalid credentials")
		return
	}
	_, err = tx.Exec(r.Context(), "UPDATE users SET failed_pin_attempts=0,pin_locked_until=NULL WHERE id=$1", id)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	s.issue(w, r, id)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	c := who(r).Claims
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `INSERT INTO revoked_tokens(jti,expires_at) VALUES($1,$2) ON CONFLICT DO NOTHING`, c.TokenID, time.Unix(c.ExpiresAt, 0))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if c.SessionID != "" {
		_, err = tx.Exec(r.Context(), `UPDATE auth_sessions SET revoked_at=now(),revocation_reason='logout'
  WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, c.SessionID, c.Subject)
		if err != nil {
			fail(w, 503, "unavailable")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "logged_out"})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	var id, phone, name string
	err := s.db.QueryRow(r.Context(), "SELECT id,phone,full_name FROM users WHERE id=$1", who(r).UserID).Scan(&id, &phone, &name)
	if err != nil {
		fail(w, 404, "not found")
		return
	}
	write(w, 200, map[string]string{"id": id, "phone": phone, "full_name": name})
}
func (s *Server) role(r *http.Request, roles ...string) (string, bool) {
	org := r.PathValue("org")
	if !validID(org) {
		return "", false
	}
	var role string
	if err := s.db.QueryRow(r.Context(), "SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2", org, who(r).UserID).Scan(&role); err != nil {
		return "", false
	}
	for _, v := range roles {
		if role == v {
			return role, true
		}
	}
	return role, false
}
func (s *Server) createOrganization(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Country string `json:"country"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Kind == "" {
		in.Kind = "company"
	}
	if in.Country == "" {
		in.Country = "GB"
	}
	in.Country = strings.ToUpper(in.Country)
	if len(in.Name) < 1 || len(in.Name) > 120 || (in.Kind != "individual" && in.Kind != "company") ||
		len(in.Country) != 2 || in.Country[0] < 'A' || in.Country[0] > 'Z' || in.Country[1] < 'A' || in.Country[1] > 'Z' {
		fail(w, 400, "invalid organization details")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), "INSERT INTO organizations(name,created_by,kind,country) VALUES($1,$2,$3,$4) RETURNING id", in.Name, who(r).UserID, in.Kind, in.Country).Scan(&id)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), "INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,'owner')", id, who(r).UserID)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 201, map[string]string{"id": id, "name": in.Name, "role": "owner", "kind": in.Kind, "country": in.Country})
}
func (s *Server) listOrganizations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), "SELECT o.id,o.name,m.role,o.kind,o.country FROM organizations o JOIN organization_members m ON o.id=m.organization_id WHERE m.user_id=$1 ORDER BY o.created_at DESC LIMIT 100", who(r).UserID)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, name, role, kind, country string
		if rows.Scan(&id, &name, &role, &kind, &country) != nil {
			fail(w, 503, "unavailable")
			return
		}
		out = append(out, map[string]string{"id": id, "name": name, "role": role, "kind": kind, "country": country})
	}
	if rows.Err() != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]any{"items": out})
}
func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	var in struct {
		Name    string         `json:"name"`
		Address map[string]any `json:"address"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 160 {
		fail(w, 400, "invalid name")
		return
	}
	addr, _ := json.Marshal(in.Address)
	if in.Address == nil {
		addr = []byte("{}")
	}
	var id string
	var version int64
	var updated time.Time
	err := s.db.QueryRow(r.Context(), "INSERT INTO properties(organization_id,name,address) VALUES($1,$2,$3) RETURNING id,version,updated_at", r.PathValue("org"), in.Name, addr).Scan(&id, &version, &updated)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 201, map[string]any{"id": id, "name": in.Name, "address": in.Address, "version": version, "updated_at": updated})
}
func (s *Server) listProperties(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	limit, created, cursorID, err := pageOptions(r, "properties", 100, 200)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,name,address,version,updated_at,created_at FROM properties
 WHERE organization_id=$1 AND deleted_at IS NULL AND (created_at,id)<($2,$3::uuid)
 ORDER BY created_at DESC,id DESC LIMIT $4`, r.PathValue("org"), created, cursorID, limit+1)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	next := ""
	more := false
	for rows.Next() {
		var id, name string
		var addr []byte
		var version int64
		var updated, createdAt time.Time
		if err = rows.Scan(&id, &name, &addr, &version, &updated, &createdAt); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		if len(out) == limit {
			more = true
			break
		}
		out = append(out, map[string]any{"id": id, "name": name, "address": json.RawMessage(addr), "version": version, "updated_at": updated})
		next = encodeSeekCursor(r.PathValue("org"), "properties", createdAt, id)
	}
	if rows.Err() != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]any{"items": out, "next_cursor": next, "has_more": more})
}
func (s *Server) createInspection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	var in struct {
		PropertyID string `json:"property_id"`
		Type       string `json:"type"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validID(in.PropertyID) || (in.Type != "move_in" && in.Type != "move_out" && in.Type != "turnover" && in.Type != "maintenance") {
		fail(w, 400, "invalid inspection")
		return
	}
	var id string
	var version int64
	var updated time.Time
	err := s.db.QueryRow(r.Context(), "INSERT INTO inspections(organization_id,property_id,inspection_type,created_by) SELECT organization_id,id,$3,$4 FROM properties WHERE organization_id=$1 AND id=$2 AND deleted_at IS NULL RETURNING id,version,updated_at", r.PathValue("org"), in.PropertyID, in.Type, who(r).UserID).Scan(&id, &version, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "property not found")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 201, map[string]any{"id": id, "property_id": in.PropertyID, "type": in.Type, "status": "draft", "version": version, "updated_at": updated})
}
func (s *Server) addItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	if !validID(r.PathValue("inspection")) {
		fail(w, 400, "invalid inspection ID")
		return
	}
	var in struct {
		ClientID string `json:"client_id"`
		Title    string `json:"title"`
		Notes    string `json:"notes"`
		Severity string `json:"severity"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if !validID(in.ClientID) || len(in.Title) < 1 || len(in.Title) > 500 || len(in.Notes) > 10000 {
		fail(w, 400, "invalid item")
		return
	}
	if in.Severity == "" {
		in.Severity = "normal"
	}
	if in.Severity != "low" && in.Severity != "normal" && in.Severity != "high" && in.Severity != "critical" {
		fail(w, 400, "invalid severity")
		return
	}
	var id string
	var version int64
	var updated time.Time
	err := s.db.QueryRow(r.Context(), "INSERT INTO inspection_items(organization_id,inspection_id,client_id,title,notes,severity) SELECT organization_id,id,$3,$4,$5,$6 FROM inspections WHERE organization_id=$1 AND id=$2 AND status IN ('draft','in_progress') AND deleted_at IS NULL ON CONFLICT (inspection_id,client_id) DO NOTHING RETURNING id,version,updated_at", r.PathValue("org"), r.PathValue("inspection"), in.ClientID, in.Title, in.Notes, in.Severity).Scan(&id, &version, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.db.QueryRow(r.Context(), `SELECT id,version,updated_at FROM inspection_items
   WHERE organization_id=$1 AND inspection_id=$2 AND client_id=$3
   AND title=$4 AND notes=$5 AND severity=$6 AND deleted_at IS NULL`,
			r.PathValue("org"), r.PathValue("inspection"), in.ClientID, in.Title, in.Notes, in.Severity).Scan(&id, &version, &updated)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 409, "inspection missing, closed or client_id conflicts")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 201, map[string]any{"id": id, "client_id": in.ClientID, "title": in.Title, "severity": in.Severity, "version": version, "updated_at": updated})
}
