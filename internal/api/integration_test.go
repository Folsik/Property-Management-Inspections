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
	"strings"
	"testing"
	"time"

	"github.com/Folsik/Property-Management-Inspections/internal/api"
	"github.com/Folsik/Property-Management-Inspections/internal/auth"
	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func TestPostgresSmoke(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("set RUN_DB_INTEGRATION=1 with an isolated PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("fresh database migration: %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("idempotent database migration: %v", err)
	}
	var versions int
	if err := db.DB.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&versions); err != nil || versions != 2 {
		t.Fatalf("migration versions: count=%d err=%v", versions, err)
	}
	tokens, err := auth.NewManager("integration-test", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.New(db.DB, tokens, slog.New(slog.NewTextHandler(io.Discard, nil)), "development").Handler())
	defer server.Close()

	call := func(method, path, token string, request any, expected int) map[string]any {
		t.Helper()
		var body io.Reader
		if request != nil {
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			body = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != expected {
			t.Fatalf("%s %s: expected %d got %d: %#v", method, path, expected, resp.StatusCode, out)
		}
		return out
	}
	register := func(phone string) string {
		t.Helper()
		otp := call("POST", "/v1/auth/otp/request", "", map[string]any{"phone": phone}, 202)
		code, _ := otp["dev_code"].(string)
		login := call("POST", "/v1/auth/otp/verify", "", map[string]any{"phone": phone, "code": code}, 200)
		token, _ := login["access_token"].(string)
		if token == "" {
			t.Fatal("no bearer token")
		}
		return token
	}
	// Unique numbers so reruns against the same test DB don't collide with OTP throttling.
	var suffix int64 = time.Now().UnixNano() % 100000000
	ownerPhone := fmt.Sprintf("+1555%08d", suffix)
	inspectorPhone := fmt.Sprintf("+1666%08d", suffix)
	outsiderPhone := fmt.Sprintf("+1777%08d", suffix)
	owner := register(ownerPhone)
	inspector := register(inspectorPhone)
	outsider := register(outsiderPhone)

	call("POST", "/v1/auth/pin", owner, map[string]any{"pin": "918245"}, 200)
	call("POST", "/v1/auth/pin", owner, map[string]any{"pin": "918245"}, 409)
	pinLogin := call("POST", "/v1/auth/pin/login", "", map[string]any{"phone": ownerPhone, "pin": "918245"}, 200)
	if pinLogin["access_token"] == "" {
		t.Fatal("pin login did not return a token")
	}
	org := call("POST", "/v1/organizations", owner, map[string]any{"name": "Test Org"}, 201)["id"].(string)
	call("GET", "/v1/organizations", owner, nil, 200)
	call("GET", "/v1/organizations/"+org+"/properties", outsider, nil, 403)
	call("POST", "/v1/organizations/"+org+"/members", owner, map[string]any{"phone": inspectorPhone, "role": "inspector"}, 201)
	call("POST", "/v1/organizations/"+org+"/members", owner, map[string]any{"phone": inspectorPhone, "role": "manager"}, 409)
	call("POST", "/v1/organizations/"+org+"/members", inspector, map[string]any{"phone": outsiderPhone, "role": "manager"}, 403)
	call("POST", "/v1/organizations/"+org+"/properties", inspector, map[string]any{"name": "Forbidden"}, 403)

	property := call("POST", "/v1/organizations/"+org+"/properties", owner, map[string]any{"name": "Apartment A", "address": map[string]any{"city": "London"}}, 201)["id"].(string)
	call("GET", "/v1/organizations/"+org+"/properties", inspector, nil, 200)
	inspection := call("POST", "/v1/organizations/"+org+"/inspections", inspector, map[string]any{"property_id": property, "type": "turnover"}, 201)["id"].(string)
	clientID := "3a3fba0c-5e94-494f-871c-7e5d8f8e7757"
	itemRequest := map[string]any{"client_id": clientID, "title": "Leaking tap", "severity": "high"}
	call("POST", "/v1/organizations/"+org+"/inspections/"+inspection+"/items", inspector, itemRequest, 201)
	call("POST", "/v1/organizations/"+org+"/inspections/"+inspection+"/items", outsider, itemRequest, 403)

	// Read paths and status transitions used by the Next.js workspace.
	inspections := call("GET", "/v1/organizations/"+org+"/inspections", owner, nil, 200)
	if len(inspections["items"].([]any)) != 1 {
		t.Fatalf("expected one live inspection: %#v", inspections)
	}
	call("GET", "/v1/organizations/"+org+"/inspections/"+inspection, owner, nil, 200)
	call("GET", "/v1/organizations/"+org+"/inspections/"+inspection, outsider, nil, 403)
	issues := call("GET", "/v1/organizations/"+org+"/inspections/"+inspection+"/items", inspector, nil, 200)
	if len(issues["items"].([]any)) != 1 {
		t.Fatalf("expected one live punch-list item: %#v", issues)
	}
	itemID := issues["items"].([]any)[0].(map[string]any)["id"].(string)
	call("PATCH", "/v1/organizations/"+org+"/inspections/"+inspection+"/items/"+itemID, inspector, map[string]any{"status": "resolved"}, 200)
	call("GET", "/v1/organizations/"+org+"/inspections/"+inspection+"/media", owner, nil, 200)
	team := call("GET", "/v1/organizations/"+org+"/members", owner, nil, 200)
	if len(team["items"].([]any)) != 2 {
		t.Fatalf("expected owner and inspector: %#v", team)
	}
	call("GET", "/v1/organizations/"+org+"/members", outsider, nil, 403)
	call("GET", "/v1/organizations/"+org+"/jobs", owner, nil, 200)
	call("PATCH", "/v1/me", owner, map[string]any{"full_name": "Property Owner"}, 200)
	call("PATCH", "/v1/organizations/"+org+"/inspections/"+inspection, inspector, map[string]any{"status": "in_progress"}, 200)
	call("PATCH", "/v1/organizations/"+org+"/inspections/"+inspection, inspector, map[string]any{"status": "completed"}, 200)
	call("PATCH", "/v1/organizations/"+org+"/inspections/"+inspection, inspector, map[string]any{"status": "draft"}, 400)
	call("PATCH", "/v1/organizations/"+org+"/inspections/"+inspection+"/items/"+itemID, inspector, map[string]any{"status": "open"}, 409)

	call("POST", "/v1/organizations/"+org+"/inspections/"+inspection+"/media/presign", inspector, map[string]any{"mime_type": "image/jpeg", "size_bytes": 2000}, 503)
	loginToken := pinLogin["access_token"].(string)
	call("POST", "/v1/auth/logout", loginToken, map[string]any{}, 200)
	call("GET", "/v1/me", loginToken, nil, 401)
	if strings.TrimSpace(org) == "" {
		t.Fatal("missing org")
	}
}
