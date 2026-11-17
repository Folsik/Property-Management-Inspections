package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"
)

// PostgreSQL-backed fixed-window admission control; shared across API replicas.
// RemoteAddr is used intentionally: an arbitrary X-Forwarded-For supplied by a
// client must never bypass throttling. Configure real-client rate limits at
// the trusted ingress/WAF in front of the Next.js BFF.
func (s *Server) limitAuth(route string, maximum int, window time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip != nil {
			if ipv4 := ip.To4(); ipv4 != nil {
				host = ipv4.String()
			} else {
				host = ip.Mask(net.CIDRMask(64, 128)).String()
			}
		}
		scope := sha256.Sum256([]byte(route + "|" + strings.TrimSpace(host)))
		key := hex.EncodeToString(scope[:])
		var hits int
		// Single-statement upsert keeps the counter atomic across Go processes.
		err = s.db.QueryRow(r.Context(), `INSERT INTO auth_rate_limits(scope_hash,window_started_at,hit_count)
  VALUES($1,now(),1)
  ON CONFLICT(scope_hash) DO UPDATE SET
    hit_count=CASE WHEN auth_rate_limits.window_started_at<now()-make_interval(secs=>$2::int)
      THEN 1 ELSE auth_rate_limits.hit_count+1 END,
    window_started_at=CASE WHEN auth_rate_limits.window_started_at<now()-make_interval(secs=>$2::int)
      THEN now() ELSE auth_rate_limits.window_started_at END
  RETURNING hit_count`, key, int(window.Seconds())).Scan(&hits)
		if err != nil {
			fail(w, 503, "rate limit unavailable")
			return
		}
		if hits > maximum {
			w.Header().Set("Retry-After", "60")
			fail(w, 429, "too many requests; try again later")
			return
		}
		next(w, r)
	}
}
