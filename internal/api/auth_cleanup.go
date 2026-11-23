package api

import (
	"context"
	"fmt"
)

// Bounded batches preserve recent challenge issuance history used by throttles.
// SKIP LOCKED allows multiple API replicas to collect without waiting on auth.
func (s *Server) CleanupExpiredAuth(ctx context.Context) (int64, error) {
	queries := []string{
		"UPDATE used_refresh_tokens SET replay_response=NULL WHERE token_hash IN (SELECT token_hash FROM used_refresh_tokens WHERE replay_response IS NOT NULL AND replay_until<now() LIMIT 1000 FOR UPDATE SKIP LOCKED)",
		"DELETE FROM used_refresh_tokens WHERE token_hash IN (SELECT token_hash FROM used_refresh_tokens WHERE expires_at<now() LIMIT 1000 FOR UPDATE SKIP LOCKED)",
		"DELETE FROM revoked_tokens WHERE jti IN (SELECT jti FROM revoked_tokens WHERE expires_at<now() LIMIT 1000 FOR UPDATE SKIP LOCKED)",
		"DELETE FROM auth_rate_limits WHERE scope_hash IN (SELECT scope_hash FROM auth_rate_limits WHERE window_started_at<now()-interval '1 hour' LIMIT 1000 FOR UPDATE SKIP LOCKED)",
		"DELETE FROM otp_challenges WHERE id IN (SELECT id FROM otp_challenges WHERE expires_at<now()-interval '1 hour' LIMIT 1000 FOR UPDATE SKIP LOCKED)",
		"DELETE FROM email_challenges WHERE id IN (SELECT id FROM email_challenges WHERE expires_at<now()-interval '1 hour' LIMIT 1000 FOR UPDATE SKIP LOCKED)",
		"DELETE FROM password_recoveries WHERE id IN (SELECT id FROM password_recoveries WHERE expires_at<now()-interval '1 hour' LIMIT 1000 FOR UPDATE SKIP LOCKED)",
		"DELETE FROM auth_sessions WHERE id IN (SELECT id FROM auth_sessions WHERE expires_at<now()-interval '1 day' LIMIT 1000 FOR UPDATE SKIP LOCKED)",
	}
	var total int64
	for i, query := range queries {
		tag, err := s.db.Exec(ctx, query)
		if err != nil {
			return total, fmt.Errorf("auth cleanup batch %d: %w", i, err)
		}
		total += tag.RowsAffected()
	}
	return total, nil
}
