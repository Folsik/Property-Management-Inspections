package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func recoveryDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func randomRecoveryToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
func recoveryReady(env string) bool {
	if env == "development" {
		return true
	}
	return os.Getenv("RESEND_API_KEY") != "" && os.Getenv("EMAIL_FROM") != "" &&
		twilioConfigFromEnv().configured() && strings.HasPrefix(os.Getenv("PUBLIC_WEB_URL"), "https://")
}
func (s *Server) requestPasswordRecovery(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	email := normalizedEmail(in.Email)
	if email == "" {
		fail(w, 400, "invalid email")
		return
	}
	if !s.limitIdentifier(w, r, "recovery-request", email, 5, 30*time.Minute) {
		return
	}
	if !recoveryReady(s.env) {
		fail(w, 503, "password recovery temporarily unavailable")
		return
	}
	const generic = "If an eligible account exists, recovery instructions have been sent."
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var user, phone string
	err = tx.QueryRow(r.Context(), `SELECT id,phone FROM users
 WHERE lower(email)=$1 AND email_verified_at IS NOT NULL AND password_hash IS NOT NULL
 FOR UPDATE`, email).Scan(&user, &phone)
	if err == pgx.ErrNoRows {
		// A dummy hash operation makes the unknown-user path less distinguishable.
		_ = verifyPassword(dummyPasswordHash, "not the real password")
		write(w, 202, map[string]string{"status": "accepted", "message": generic})
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	var count int
	err = tx.QueryRow(r.Context(), `SELECT count(*) FROM password_recoveries
 WHERE user_id=$1 AND created_at>now()-interval '30 minutes'`, user).Scan(&count)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if count >= 3 {
		write(w, 202, map[string]string{"status": "accepted", "message": generic})
		return
	}
	token, err := randomRecoveryToken()
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	// Invalidate previous outstanding challenges before issuing a new one.
	_, err = tx.Exec(r.Context(), `UPDATE auth_sessions SET revoked_at=now(),revocation_reason='password_reset'
 WHERE user_id=$1 AND revoked_at IS NULL`, user)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE password_recoveries SET used_at=now() WHERE user_id=$1 AND used_at IS NULL`, user)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO password_recoveries(user_id,token_hash,sms_code_hash,expires_at)
 VALUES($1,$2,$3,now()+interval '10 minutes')`, user, recoveryDigest(token), string(hash))
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	origin := strings.TrimRight(os.Getenv("PUBLIC_WEB_URL"), "/")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	link := origin + "/reset-password?token=" + token
	emailConfigured := os.Getenv("RESEND_API_KEY") != "" && os.Getenv("EMAIL_FROM") != ""
	sms := twilioConfigFromEnv()
	if emailConfigured {
		body := "<p>To reset your Inspectly password, follow this link:</p><p><a href=\"" + html.EscapeString(link) + "\">Reset password</a></p><p>Expires in 10 minutes. A separate code will be sent by SMS. If you did not request this, ignore this email.</p>"
		if err = sendTransactionalEmail(r.Context(), email, "Reset your Inspectly password", body); err != nil {
			s.logger.Error("recovery email delivery failed", "error", err)
			fail(w, 503, "recovery delivery unavailable")
			return
		}
	}
	if sms.configured() {
		if err = sendTwilioSMS(r.Context(), sms, phone, "Inspectly password recovery code: "+code+". Expires in 10 minutes. Never share this code."); err != nil {
			s.logger.Error("recovery SMS delivery failed", "error", err)
			fail(w, 503, "recovery delivery unavailable")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	response := map[string]string{"status": "accepted", "message": generic}
	if s.env == "development" {
		if !emailConfigured {
			response["dev_token"] = token
		}
		if !sms.configured() {
			response["dev_code"] = code
		}
	}
	write(w, 202, response)
}

func (s *Server) completePasswordRecovery(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Token) != 64 || len(in.Code) != 6 || len(in.Password) > 1024 {
		fail(w, 401, "invalid or expired recovery details")
		return
	}
	if _, err := hex.DecodeString(in.Token); err != nil {
		fail(w, 401, "invalid or expired recovery details")
		return
	}
	for _, c := range in.Code {
		if c < '0' || c > '9' {
			fail(w, 401, "invalid or expired recovery details")
			return
		}
	}
	// Hash outside the recovery transaction; no row lock spans Argon2.
	prehashed, err := hashPassword(in.Password)
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
	var id, user, hash string
	var attempts int
	err = tx.QueryRow(r.Context(), `SELECT id,user_id,sms_code_hash,attempts FROM password_recoveries
 WHERE token_hash=$1 AND used_at IS NULL AND expires_at>now()
 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, recoveryDigest(in.Token)).Scan(&id, &user, &hash, &attempts)
	if err == pgx.ErrNoRows {
		fail(w, 401, "invalid or expired recovery details")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if attempts >= 5 {
		fail(w, 429, "recovery code temporarily locked")
		return
	}
	good := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Code)) == nil
	if !good {
		_, err = tx.Exec(r.Context(), `UPDATE password_recoveries SET attempts=attempts+1 WHERE id=$1`, id)
		if err != nil {
			fail(w, 503, "unavailable")
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		fail(w, 401, "invalid or expired recovery details")
		return
	}
	var phone, email string
	err = tx.QueryRow(r.Context(), `SELECT phone,email FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&phone, &email)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = validatePassword(in.Password, phone, email); err != nil {
		fail(w, 400, err.Error())
		return
	}
	nextHash := prehashed
	_, err = tx.Exec(r.Context(), `UPDATE users SET password_hash=$2,auth_epoch=auth_epoch+1,
  failed_password_attempts=0,password_locked_until=NULL,updated_at=now() WHERE id=$1`, user, nextHash)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE password_recoveries SET used_at=now() WHERE user_id=$1 AND used_at IS NULL`, user)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	// Do not grant a new session during recovery.
	if os.Getenv("RESEND_API_KEY") != "" && os.Getenv("EMAIL_FROM") != "" {
		if err = sendTransactionalEmail(r.Context(), email, "Your Inspectly password was reset",
			"<p>Your Inspectly password has been changed. If this wasn't you, contact support immediately.</p>"); err != nil {
			s.logger.Error("recovery confirmation email failed", "error", err)
		}
	}
	s.auditSecurityEvent(r, user, "", "auth.password_reset", "user", user)
	write(w, 200, map[string]string{"status": "password_reset", "message": "Your password was updated. Sign in again."})
}
