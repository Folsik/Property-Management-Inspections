package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/unicode/norm"
)

// Exact-match denylist. The user-provided password is never rejected just for
// containing a dictionary word. Extend the dataset with breached corpus checks.
var commonPasswords = map[string]bool{
	"password": true, "password123": true, "password123!": true, "qwerty": true,
	"123456789012345": true, "qwertyqwertyqwerty": true,
	"passwordpassword": true, "correct horse battery staple": true,
	"letmein": true, "iloveyou": true, "welcome123": true, "admin123": true,
	"123456789": true, "password123456789": true, "12345678901234567890": true,
	"the quick brown fox jumps over the lazy dog": true,
}

func validatePassword(value, phone, email string) error {
	if !utf8.ValidString(value) {
		return errors.New("invalid UTF-8 password")
	}
	normalized := norm.NFC.String(value)
	runeCount := utf8.RuneCountInString(normalized)
	if runeCount < 15 || runeCount > 128 || len(normalized) > 1024 {
		return errors.New("Use a password of 15 to 128 characters (up to 1024 UTF-8 bytes)")
	}
	for _, r := range normalized {
		if unicode.IsControl(r) {
			return errors.New("Password cannot contain control characters")
		}
	}
	lower := strings.ToLower(normalized)
	if commonPasswords[lower] {
		return errors.New("This password is commonly used; choose a different one")
	}
	if email != "" && lower == strings.ToLower(email) {
		return errors.New("Password cannot be your email address")
	}
	if phone != "" && normalized == phone {
		return errors.New("Password cannot be your phone number")
	}
	return nil
}

func hashPassword(value string) (string, error) {
	normalized := []byte(norm.NFC.String(value))
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	// OWASP Argon2id parameter floor: 19 MiB memory, 2 passes, 1 lane.
	digest := argon2.IDKey(normalized, salt, 2, 19*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(digest)), nil
}

func verifyPassword(hash, value string) bool {
	if strings.HasPrefix(hash, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value)) == nil
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=19456,t=2,p=1" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	target, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(target) != 32 {
		return false
	}
	if !utf8.ValidString(value) || len(value) > 1024 {
		return false
	}
	got := argon2.IDKey([]byte(norm.NFC.String(value)), salt, 2, 19*1024, 1, 32)
	return subtle.ConstantTimeCompare(got, target) == 1
}

// Material used only for the nonexistent-identifier comparison path.
var dummyPasswordHash = func() string { hash, _ := hashPassword("randomly generated nonexistent-user secret"); return hash }()

func (s *Server) setPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	var phone string
	var email *string
	if err := s.db.QueryRow(r.Context(), `SELECT phone,email FROM users WHERE id=$1`, who(r).UserID).Scan(&phone, &email); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	currentEmail := ""
	if email != nil {
		currentEmail = *email
	}
	if err := validatePassword(in.Password, phone, currentEmail); err != nil {
		fail(w, 400, err.Error())
		return
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE users SET password_hash=$2,failed_password_attempts=0,
 password_locked_until=NULL,updated_at=now() WHERE id=$1 AND password_hash IS NULL`, who(r).UserID, hash)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "password already set")
		return
	}
	write(w, 200, map[string]string{"status": "password_set"})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var phone, hash string
	var email *string
	err = tx.QueryRow(r.Context(), `SELECT phone,coalesce(password_hash,''),email FROM users WHERE id=$1 FOR UPDATE`, who(r).UserID).Scan(&phone, &hash, &email)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if hash == "" || !verifyPassword(hash, in.CurrentPassword) {
		fail(w, 401, "invalid current password")
		return
	}
	if verifyPassword(hash, in.NewPassword) {
		fail(w, 400, "choose a different password")
		return
	}
	currentEmail := ""
	if email != nil {
		currentEmail = *email
	}
	if err := validatePassword(in.NewPassword, phone, currentEmail); err != nil {
		fail(w, 400, err.Error())
		return
	}
	nextHash, err := hashPassword(in.NewPassword)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE users SET password_hash=$2,failed_password_attempts=0,
 password_locked_until=NULL,updated_at=now() WHERE id=$1`, who(r).UserID, nextHash)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "password_changed"})
}

func (s *Server) passwordLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	identifier := strings.TrimSpace(in.Identifier)
	if len(identifier) < 3 || len(identifier) > 254 || len(in.Password) == 0 || len(in.Password) > 1024 {
		fail(w, 401, "invalid credentials")
		return
	}
	if strings.Contains(identifier, "@") {
		identifier = normalizedEmail(identifier)
		if identifier == "" {
			fail(w, 401, "invalid credentials")
			return
		}
	} else if !phonePattern.MatchString(identifier) {
		fail(w, 401, "invalid credentials")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var id, hash string
	var locked *time.Time
	err = tx.QueryRow(r.Context(), `SELECT id,coalesce(password_hash,''),password_locked_until FROM users
 WHERE phone=$1 OR (lower(email)=$1 AND email_verified_at IS NOT NULL) FOR UPDATE`, identifier).Scan(&id, &hash, &locked)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = verifyPassword(dummyPasswordHash, in.Password)
		fail(w, 401, "invalid credentials")
		return
	}
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if locked != nil && locked.After(time.Now()) {
		fail(w, 429, "temporarily locked; try again later")
		return
	}
	if hash == "" || !verifyPassword(hash, in.Password) {
		_, err = tx.Exec(r.Context(), `UPDATE users SET failed_password_attempts=CASE WHEN failed_password_attempts>=4 THEN 0 ELSE failed_password_attempts+1 END,
  password_locked_until=CASE WHEN failed_password_attempts>=4 THEN now()+interval '15 minutes' ELSE NULL END WHERE id=$1`, id)
		if err != nil {
			fail(w, 503, "unavailable")
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		fail(w, 401, "invalid credentials")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE users SET failed_password_attempts=0,password_locked_until=NULL WHERE id=$1`, id)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	s.issue(w, id)
}
