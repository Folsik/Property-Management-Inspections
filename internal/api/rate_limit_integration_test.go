package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Folsik/Property-Management-Inspections/internal/config"
	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func TestDistributedAuthRateLimitDoesNotTrustSpoofedForwardedIP(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s := New(db.DB, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "development")
	route := fmt.Sprintf("integration-rate-%d", time.Now().UnixNano())
	handler := s.limitAuth(route, 2, time.Minute, func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]bool{"ok": true}) })
	first := httptest.NewServer(handler)
	defer first.Close()
	second := httptest.NewServer(handler)
	defer second.Close()
	for i := 0; i < 3; i++ {
		address := first.URL
		if i == 2 {
			address = second.URL
		}
		req, err := http.NewRequestWithContext(ctx, "POST", address+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.12.4.%d", i+1))
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		wanted := 200
		if i == 2 {
			wanted = 429
		}
		if response.StatusCode != wanted {
			t.Fatalf("request %d: want %d got %d", i, wanted, response.StatusCode)
		}
	}
}

func TestTrustedBFFAndIdentifierRateLimits(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	first := New(db.DB, nil, logger, "development")
	second := New(db.DB, nil, logger, "development")
	secret := strings.Repeat("ab", 32)
	for _, s := range []*Server{first, second} {
		if err := s.ConfigureTrustedBFF(secret); err != nil {
			t.Fatal(err)
		}
	}
	call := func(h http.Handler, path, ip, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(ctx)
		req.RemoteAddr = "172.18.0.7:12345"
		req.Header.Set(proxySecretHeader, secret)
		req.Header.Set(clientIPHeader, ip)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s: want %d, got %d: %s", path, want, w.Code, w.Body.String())
		}
		if want == 429 && w.Header().Get("Retry-After") == "" {
			t.Fatal("missing Retry-After")
		}
	}
	stamp := time.Now().UnixNano()
	route := fmt.Sprintf("trusted-proxy-%d", stamp)
	next := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
	a := first.limitAuth(route, 2, time.Minute, next)
	b := second.limitAuth(route, 2, time.Minute, next)
	call(a, "/", "192.0.2.1", "", 200)
	call(b, "/", "192.0.2.1", "", 200)
	call(a, "/", "192.0.2.2", "", 200)
	call(b, "/", "192.0.2.1", "", 429)

	// The real login endpoint must throttle the same normalized identifier
	// across separate IP addresses and independent API instances.
	email := fmt.Sprintf("missing-%d@example.test", stamp)
	h1, h2 := first.Handler(), second.Handler()
	for i := 0; i < 20; i++ {
		h := h1
		value := email
		if i%2 == 1 {
			h = h2
			value = " " + strings.ToUpper(email) + " "
		}
		body := fmt.Sprintf(`{"identifier":%q,"password":"not the real password"}`, value)
		ip := fmt.Sprintf("2001:db8:%x:%x::1", uint16(stamp>>16), uint16(stamp)+uint16(i))
		call(h, "/v1/auth/password/login", ip, body, 401)
	}
	body := fmt.Sprintf(`{"identifier":%q,"password":"not the real password"}`, email)
	call(h2, "/v1/auth/password/login", "198.51.100.11", body, 429)
	other := fmt.Sprintf(`{"identifier":%q,"password":"not the real password"}`, "other-"+email)
	call(h1, "/v1/auth/password/login", "198.51.100.11", other, 401)
}

func TestDefaultProductionOTPDoesNotLeakCode(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires PostgreSQL")
	}
	t.Setenv("APP_ENV", "")
	t.Setenv("TWILIO_ACCOUNT_SID", "")
	t.Setenv("TWILIO_AUTH_TOKEN", "")
	t.Setenv("TWILIO_FROM_NUMBER", "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s := New(db.DB, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), config.Load().Env)
	phone := fmt.Sprintf("+1999%08d", time.Now().UnixNano()%100000000)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/request", strings.NewReader(fmt.Sprintf(`{"phone":%q}`, phone))).WithContext(ctx)
	req.RemoteAddr = "203.0.113.15:1234"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 503 || strings.Contains(w.Body.String(), "dev_code") {
		t.Fatalf("production must fail closed without SMS provider: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err := db.DB.QueryRow(ctx, "SELECT count(*) FROM otp_challenges WHERE phone=$1", phone).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unexpected OTP challenge: count=%d err=%v", count, err)
	}
}
