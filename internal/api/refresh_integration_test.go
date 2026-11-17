package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Folsik/Property-Management-Inspections/internal/api"
	"github.com/Folsik/Property-Management-Inspections/internal/auth"
	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func TestRefreshRotationDeviceRevocation(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	tm, err := auth.NewManager("refresh-integration", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.New(db.DB, tm, slog.New(slog.NewTextHandler(io.Discard, nil)), "development").Handler())
	defer srv.Close()
	call := func(method, path, access string, body any, want int) map[string]any {
		t.Helper()
		var input io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			input = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(ctx, method, srv.URL+path, input)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if access != "" {
			req.Header.Set("Authorization", "Bearer "+access)
		}
		req.Header.Set("User-Agent", "Inspectly-Integration-iPhone")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var data map[string]any
		if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s %s wanted %d got %d: %#v", method, path, want, res.StatusCode, data)
		}
		return data
	}
	stamp := time.Now().UnixNano() % 100000000
	phone := fmt.Sprintf("+1888%08d", stamp)
	otp := call("POST", "/v1/auth/otp/request", "", map[string]string{"phone": phone}, 202)
	first := call("POST", "/v1/auth/otp/verify", "", map[string]string{"phone": phone, "code": otp["dev_code"].(string)}, 200)
	a1 := first["access_token"].(string)
	r1 := first["refresh_token"].(string)
	if r1 == "" || first["session_id"] == "" {
		t.Fatal("missing refresh session")
	}
	call("GET", "/v1/me", a1, nil, 200)
	newPair := call("POST", "/v1/auth/refresh", "", map[string]string{"refresh_token": r1}, 200)
	a2 := newPair["access_token"].(string)
	r2 := newPair["refresh_token"].(string)
	if r1 == r2 {
		t.Fatal("token did not rotate")
	}
	call("GET", "/v1/me", a2, nil, 200)
	// Strict rotation: old access is revoked by rotation (session token version).
	call("GET", "/v1/me", a1, nil, 401)
	// Presenting a used token signals theft and revokes the entire device session.
	call("POST", "/v1/auth/refresh", "", map[string]string{"refresh_token": r1}, 401)
	call("POST", "/v1/auth/refresh", "", map[string]string{"refresh_token": r2}, 401)
	call("GET", "/v1/me", a2, nil, 401)
	// Two sessions from same account are isolated by session ID.
	call("POST", "/v1/auth/password", a1, map[string]string{"password": "very long sample password for testing"}, 401)
	// Fresh legacy OTP cannot log in once password is enrolled.
	// Create a second user for the independent device management test.
	phone2 := fmt.Sprintf("+1998%08d", stamp)
	otp2 := call("POST", "/v1/auth/otp/request", "", map[string]string{"phone": phone2}, 202)
	setup := call("POST", "/v1/auth/otp/verify", "", map[string]string{"phone": phone2, "code": otp2["dev_code"].(string)}, 200)
	call("POST", "/v1/auth/password", setup["access_token"].(string), map[string]string{"password": "a completely different lengthy password"}, 200)
	dev1 := call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": phone2, "password": "a completely different lengthy password"}, 200)
	dev2 := call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": phone2, "password": "a completely different lengthy password"}, 200)
	sid1 := dev1["session_id"].(string)
	sid2 := dev2["session_id"].(string)
	if sid1 == sid2 {
		t.Fatal("different devices share a session")
	}
	list := call("GET", "/v1/auth/sessions", dev2["access_token"].(string), nil, 200)
	if len(list["items"].([]any)) < 2 {
		t.Fatalf("sessions not listed: %#v", list)
	}
	call("DELETE", "/v1/auth/sessions/"+sid1, dev2["access_token"].(string), nil, 200)
	call("GET", "/v1/me", dev1["access_token"].(string), nil, 401)
	call("POST", "/v1/auth/refresh", "", map[string]string{"refresh_token": dev1["refresh_token"].(string)}, 401)
	call("GET", "/v1/me", dev2["access_token"].(string), nil, 200)
	call("POST", "/v1/auth/refresh/logout", "", map[string]string{"refresh_token": dev2["refresh_token"].(string)}, 200)
	call("GET", "/v1/me", dev2["access_token"].(string), nil, 401)
}
