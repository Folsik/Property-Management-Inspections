package api

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var commonPasswords = map[string]bool{
	"password": true, "password123": true, "password123!": true, "password1234!": true,
	"qwerty": true, "qwerty123": true, "qwerty123!xyz": true, "welcome123": true,
	"welcome123!": true, "admin123": true, "admin123!": true, "letmein": true,
	"iloveyou": true, "changeme": true, "123456789": true, "1234567890": true,
	"1234567890ab!": true, "aaaaaaaaaaaaa1!": true,
}

func validatePassword(value, phone, email string) error {
	if len(value) < 12 || len(value) > 72 || !utf8.ValidString(value) {
		return errors.New("Password must contain 12–72 UTF-8 bytes")
	}
	var upper, lowerCase, digit, symbol bool
	for _, r := range value {
		if unicode.IsUpper(r) {
			upper = true
		}
		if unicode.IsLower(r) {
			lowerCase = true
		}
		if unicode.IsDigit(r) {
			digit = true
		}
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			symbol = true
		}
		if unicode.IsControl(r) {
			return errors.New("Password contains unsupported control characters")
		}
	}
	if !upper || !lowerCase || !digit || !symbol {
		return errors.New("Password must include upper- and lowercase letters, digits and a special character")
	}
	lower := strings.ToLower(value)
	if commonPasswords[lower] {
		return errors.New("Choose a less common password")
	}
	for _, pattern := range []string{"password", "qwerty", "123456", "654321", "abcdef", "letmein", "welcome123", "admin123", "iloveyou"} {
		if strings.Contains(lower, pattern) {
			return errors.New("Password is too easy to guess")
		}
	}
	// Detect five identical consecutive characters without regex backreferences.
	run := 1
	for i := 1; i < len(value); i++ {
		if value[i] == value[i-1] {
			run++
			if run >= 5 {
				return errors.New("Password contains a repeated sequence")
			}
		} else {
			run = 1
		}
	}
	local := strings.Split(strings.ToLower(email), "@")[0]
	if len(local) >= 4 && strings.Contains(lower, local) {
		return errors.New("Password must not contain your email name")
	}
	digits := strings.TrimPrefix(phone, "+")
	if len(digits) >= 7 && strings.Contains(lower, digits) {
		return errors.New("Password must not contain your phone number")
	}
	return nil
}

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
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE users SET password_hash=$2, failed_password_attempts=0,
 password_locked_until=NULL, updated_at=now() WHERE id=$1 AND password_hash IS NULL`, who(r).UserID, string(hash))
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
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.CurrentPassword)) != nil {
		fail(w, 401, "invalid current password")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.NewPassword)) == nil {
		fail(w, 400, "choose a different password")
		return
	}
	existingEmail := ""
	if email != nil {
		existingEmail = *email
	}
	if err := validatePassword(in.NewPassword, phone, existingEmail); err != nil {
		fail(w, 400, err.Error())
		return
	}
	nextHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE users SET password_hash=$2,failed_password_attempts=0,password_locked_until=NULL,
 updated_at=now() WHERE id=$1`, who(r).UserID, string(nextHash))
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
	if len(identifier) < 3 || len(identifier) > 254 || len(in.Password) == 0 || len(in.Password) > 72 {
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
	if errors.Is(err, pgx.ErrNoRows) { // Make missing account roughly as expensive as a wrong password.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(in.Password))
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
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		_, err = tx.Exec(r.Context(), `UPDATE users SET failed_password_attempts=
   CASE WHEN failed_password_attempts>=4 THEN 0 ELSE failed_password_attempts+1 END,
   password_locked_until=CASE WHEN failed_password_attempts>=4 THEN now()+interval '15 minutes' ELSE NULL END
   WHERE id=$1`, id)
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

// bcrypt hash of a static unreachable random password, used to avoid a fast username-enumeration path.
const dummyPasswordHash = "$2a$10$ROOskaGYq9NOqJpmrdi1bOWzuUadTBCj8Y.4.fJFBJKVnYZpGBkt."
