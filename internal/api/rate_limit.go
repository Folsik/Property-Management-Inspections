package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	clientIPHeader    = "X-Inspectly-Client-IP"
	proxySecretHeader = "X-Inspectly-Proxy-Secret"
)

// ConfigureTrustedBFF must be called before serving requests. An empty secret
// disables forwarded-IP trust; a configured secret must be 32 random bytes in hex.
func (s *Server) ConfigureTrustedBFF(secret string) error {
	if secret != "" {
		decoded, err := hex.DecodeString(secret)
		if err != nil || len(decoded) != 32 {
			return errors.New("TRUSTED_BFF_SECRET must be 64 hexadecimal characters")
		}
	}
	s.trustedBFFSecret = secret
	return nil
}

// Never trust generic forwarding headers. Only the authenticated BFF may
// assert a single bare client IP, obtained from a separately trusted ingress.
func (s *Server) clientRateLimitIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	proof := r.Header.Values(proxySecretHeader)
	forwarded := r.Header.Values(clientIPHeader)
	if s.trustedBFFSecret != "" && len(proof) == 1 && len(forwarded) == 1 &&
		subtle.ConstantTimeCompare([]byte(proof[0]), []byte(s.trustedBFFSecret)) == 1 &&
		net.ParseIP(forwarded[0]) != nil {
		host = forwarded[0]
	}
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
		return ip.Mask(net.CIDRMask(64, 128)).String()
	}
	return strings.TrimSpace(host)
}

// PostgreSQL-backed fixed-window admission control, shared across replicas.
// Preserve the existing IP scope; identifier scopes are independent of IP.
func (s *Server) limitAuth(route string, maximum int, window time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.admitAuth(w, r, route+"|"+s.clientRateLimitIP(r), maximum, window) {
			next(w, r)
		}
	}
}

func (s *Server) limitIdentifier(w http.ResponseWriter, r *http.Request, route, identifier string, maximum int, window time.Duration) bool {
	return s.admitAuth(w, r, route+"|identifier|"+identifier, maximum, window)
}

func (s *Server) admitAuth(w http.ResponseWriter, r *http.Request, scope string, maximum int, window time.Duration) bool {
	digest := sha256.Sum256([]byte(scope))
	key := hex.EncodeToString(digest[:])
	var hits int
	// Single-statement upsert keeps the counter atomic across Go processes.
	err := s.db.QueryRow(r.Context(), `INSERT INTO auth_rate_limits(scope_hash,window_started_at,hit_count)
  VALUES($1,now(),1)
  ON CONFLICT(scope_hash) DO UPDATE SET
    hit_count=CASE WHEN auth_rate_limits.window_started_at<now()-make_interval(secs=>$2::int)
      THEN 1 ELSE auth_rate_limits.hit_count+1 END,
    window_started_at=CASE WHEN auth_rate_limits.window_started_at<now()-make_interval(secs=>$2::int)
      THEN now() ELSE auth_rate_limits.window_started_at END
  RETURNING hit_count`, key, int(window.Seconds())).Scan(&hits)
	if err != nil {
		fail(w, 503, "rate limit unavailable")
		return false
	}
	if hits > maximum {
		w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
		fail(w, 429, "too many requests; try again later")
		return false
	}
	return true
}
