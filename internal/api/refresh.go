package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const refreshLifetime = 30 * 24 * time.Hour

func refreshHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func validRefreshToken(v string) bool {
	if len(v) != 43 {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(v)
	return err == nil && len(b) == 32
}
func deviceLabel(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("User-Agent"))
	if len(value) > 120 {
		value = value[:120]
	}
	if value == "" {
		return "Unknown device"
	}
	return value
}
func (s *Server) issue(w http.ResponseWriter, r *http.Request, user string) {
	ctx := r.Context()
	var epoch int64
	if err := s.db.QueryRow(ctx, `SELECT auth_epoch FROM users WHERE id=$1`, user).Scan(&epoch); err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	raw, err := newRefreshToken()
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	var sid string
	var expires time.Time
	err = s.db.QueryRow(ctx, `INSERT INTO auth_sessions(user_id,current_token_hash,auth_epoch,device_name,expires_at)
 VALUES($1,$2,$3,$4,now()+interval '30 days') RETURNING id,expires_at`,
		user, refreshHash(raw), epoch, deviceLabel(r)).Scan(&sid, &expires)
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	access, until, err := s.tokens.IssueWithSession(user, epoch, sid, 1)
	if err != nil {
		_, _ = s.db.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now(),revocation_reason='issue_failed' WHERE id=$1`, sid)
		fail(w, 503, "auth unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, map[string]any{"access_token": access, "token_type": "Bearer", "expires_at": until,
		"refresh_token": raw, "refresh_expires_at": expires, "session_id": sid})
}
// Cached responses are encrypted with a deployment-wide AEAD key. Only the
// exact token used in the previous rotation can retrieve the response.
func (s *Server) encryptRefreshResponse(value map[string]any) ([]byte, error) {
	plaintext, err := json.Marshal(value)
	if err != nil { return nil, err }
	block, err := aes.NewCipher(s.refreshReplayKey[:])
	if err != nil { return nil, err }
	aead, err := cipher.NewGCM(block)
	if err != nil { return nil, err }
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil { return nil, err }
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *Server) decryptRefreshResponse(encrypted []byte) (map[string]any, error) {
	block, err := aes.NewCipher(s.refreshReplayKey[:])
	if err != nil { return nil, err }
	aead, err := cipher.NewGCM(block)
	if err != nil { return nil, err }
	if len(encrypted) < aead.NonceSize() { return nil, errors.New("invalid encrypted refresh response") }
	plaintext, err := aead.Open(nil, encrypted[:aead.NonceSize()], encrypted[aead.NonceSize():], nil)
	if err != nil { return nil, err }
	var response map[string]any
	if err := json.Unmarshal(plaintext, &response); err != nil { return nil, err }
	return response, nil
}
func (s *Server) refreshAccess(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validRefreshToken(in.RefreshToken) {
		fail(w, 401, "invalid refresh token")
		return
	}
	digest := refreshHash(in.RefreshToken)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	// Cross-replica serialization prevents double rotations on concurrent retry.
	if _, err := tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", digest); err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	var sid, user string
	var epoch, gen int64
	var expires time.Time
	var revoked *time.Time
	err = tx.QueryRow(r.Context(), `SELECT id,user_id,auth_epoch,generation,expires_at,revoked_at
 FROM auth_sessions WHERE current_token_hash=$1 FOR UPDATE`, digest).
		Scan(&sid, &user, &epoch, &gen, &expires, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		var oldSession string
		var cached []byte
		var withinGrace bool
		err = tx.QueryRow(r.Context(), `SELECT session_id,replay_response,
 (replay_until IS NOT NULL AND replay_until>now()) FROM used_refresh_tokens WHERE token_hash=$1`, digest).
			Scan(&oldSession, &cached, &withinGrace)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 401, "invalid refresh token")
			return
		}
		if err != nil {
			fail(w, 503, "auth unavailable")
			return
		}
		if withinGrace && len(cached) > 0 {
			response, err := s.decryptRefreshResponse(cached)
			if err != nil {
				fail(w, 503, "auth unavailable")
				return
			}
			newToken, ok := response["refresh_token"].(string)
			if !ok || !validRefreshToken(newToken) {
				fail(w, 503, "auth unavailable")
				return
			}
			var active bool
			err = tx.QueryRow(r.Context(), `SELECT EXISTS(
 SELECT 1 FROM auth_sessions a JOIN users u ON u.id=a.user_id
 WHERE a.id=$1 AND a.current_token_hash=$2 AND a.revoked_at IS NULL
 AND a.expires_at>now() AND a.auth_epoch=u.auth_epoch)`,
				oldSession, refreshHash(newToken)).Scan(&active)
			if err != nil {
				fail(w, 503, "auth unavailable")
				return
			}
			if active {
				if err := tx.Commit(r.Context()); err != nil {
					fail(w, 503, "auth unavailable")
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				write(w, 200, response)
				return
			}
			// An older generation cannot revoke an already advanced session.
			fail(w, 401, "invalid refresh token")
			return
		}
		// Replay outside grace revokes the session, preserving theft detection.
		_, err = tx.Exec(r.Context(), `UPDATE auth_sessions SET revoked_at=now(),revocation_reason='replay'
 WHERE id=$1 AND revoked_at IS NULL`, oldSession)
		if err != nil {
			fail(w, 503, "auth unavailable")
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			fail(w, 503, "auth unavailable")
			return
		}
		fail(w, 401, "refresh token reuse detected; device signed out")
		return
	}
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	if revoked != nil || !expires.After(time.Now()) {
		fail(w, 401, "session expired or revoked")
		return
	}
	var activeEpoch int64
	if err = tx.QueryRow(r.Context(), `SELECT auth_epoch FROM users WHERE id=$1`, user).Scan(&activeEpoch); err != nil {
		fail(w, 401, "session expired")
		return
	}
	if activeEpoch != epoch {
		fail(w, 401, "session expired")
		return
	}
	next, err := newRefreshToken()
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	access, until, err := s.tokens.IssueWithSession(user, epoch, sid, gen+1)
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	response := map[string]any{"access_token": access, "token_type": "Bearer", "expires_at": until,
		"refresh_token": next, "refresh_expires_at": expires, "session_id": sid}
	sealed, err := s.encryptRefreshResponse(response)
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO used_refresh_tokens(token_hash,session_id,expires_at,replay_response,replay_until)
 VALUES($1,$2,$3,$4,now()+interval '20 seconds')`, digest, sid, expires, sealed)
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	tag, err := tx.Exec(r.Context(), `UPDATE auth_sessions SET current_token_hash=$2,generation=generation+1,
 last_seen_at=now() WHERE id=$1 AND revoked_at IS NULL`, sid, refreshHash(next))
	if err != nil || tag.RowsAffected() != 1 {
		fail(w, 503, "auth unavailable")
		return
	}

	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, response)
}
func (s *Server) logoutRefresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validRefreshToken(in.RefreshToken) {
		fail(w, 401, "invalid refresh token")
		return
	}
	digest := refreshHash(in.RefreshToken)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), `SELECT id FROM auth_sessions WHERE current_token_hash=$1 FOR UPDATE`, digest).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Do not let arbitrary revoked/unknown values revoke another user's session.
		fail(w, 401, "invalid refresh token")
		return
	}
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()),
 revocation_reason='logout' WHERE id=$1`, id)
	if err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "auth unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "signed_out"})
}
func (s *Server) listAuthSessions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `SELECT id,device_name,created_at,last_seen_at,expires_at
 FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now()
 ORDER BY last_seen_at DESC LIMIT 100`, who(r).UserID)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, name string
		var created, last, exp time.Time
		if err = rows.Scan(&id, &name, &created, &last, &exp); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		items = append(items, map[string]any{"id": id, "device_name": name, "created_at": created,
			"last_seen_at": last, "expires_at": exp, "current": id == who(r).Claims.SessionID})
	}
	if rows.Err() != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 200, map[string]any{"items": items})
}
func (s *Server) revokeAuthSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("session")
	if !validID(id) {
		fail(w, 400, "invalid session id")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE auth_sessions
 SET revoked_at=now(),revocation_reason='user_revoked'
 WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, who(r).UserID)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if tag.RowsAffected() != 1 {
		fail(w, 404, "session not found")
		return
	}
	write(w, 200, map[string]string{"status": "revoked"})
}
