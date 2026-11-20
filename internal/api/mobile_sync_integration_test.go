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

func TestMobileSyncConcurrencyAndIsolation(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("needs PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewManager("mobile-sync-tests", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.New(db.DB, tokens, slog.New(slog.NewTextHandler(io.Discard, nil)), "development").Handler())
	defer server.Close()
	var owner, outsider string
	stamp := time.Now().UnixNano() % 10000000000
	for i, ptr := range []*string{&owner, &outsider} {
		if err = db.DB.QueryRow(ctx, "INSERT INTO users(phone) VALUES($1) RETURNING id", fmt.Sprintf("+1444%010d", stamp+int64(i))).Scan(ptr); err != nil {
			t.Fatal(err)
		}
	}
	var org, otherOrg, property, inspection string
	for i, ptr := range []*string{&org, &otherOrg} {
		if err = db.DB.QueryRow(ctx, "INSERT INTO organizations(name,created_by) VALUES($1,$2) RETURNING id", fmt.Sprintf("Offline test org %d", i), owner).Scan(ptr); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.DB.Exec(ctx, "INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,'owner')", org, owner); err != nil {
		t.Fatal(err)
	}
	if err = db.DB.QueryRow(ctx, "INSERT INTO properties(organization_id,name) VALUES($1,'Offline flat') RETURNING id", org).Scan(&property); err != nil {
		t.Fatal(err)
	}
	if err = db.DB.QueryRow(ctx, "INSERT INTO inspections(organization_id,property_id,created_by,inspection_type) VALUES($1,$2,$3,'turnover') RETURNING id", org, property, owner).Scan(&inspection); err != nil {
		t.Fatal(err)
	}
	token, _, err := tokens.Issue(owner)
	if err != nil {
		t.Fatal(err)
	}
	otherToken, _, err := tokens.Issue(outsider)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, bearer string, input any, want int) map[string]any {
		t.Helper()
		var body io.Reader
		if input != nil {
			data, _ := json.Marshal(input)
			body = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
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
			t.Fatalf("%s %s expected %d got %d: %v", method, path, want, res.StatusCode, data)
		}
		return data
	}
	base := "/v1/organizations/" + org
	items := base + "/inspections/" + inspection + "/items"
	cid := "00000000-aaaa-4bbb-8ccc-000000000001"
	created := call("POST", items, token, map[string]any{"client_id": cid, "title": "Bathroom check", "notes": "initial", "severity": "normal"}, 201)
	id := created["id"].(string)
	identical := call("POST", items, token, map[string]any{"client_id": cid, "title": "Bathroom check", "notes": "initial", "severity": "normal"}, 201)
	if identical["id"] != id || identical["version"] != float64(1) {
		t.Fatalf("idempotent REST create should retain id/version: %v", identical)
	}

	current := call("GET", items+"?limit=1", token, nil, 200)
	first := current["items"].([]any)[0].(map[string]any)
	version := first["version"].(float64)
	if version != 1 {
		t.Fatalf("version after insert: %v", first)
	}
	call("PATCH", items+"/"+id, token, map[string]any{"status": "resolved", "version": int(version)}, 200)
	stale := call("PATCH", items+"/"+id, token, map[string]any{"status": "open", "version": int(version)}, 409)
	if stale["current"] == nil {
		t.Fatalf("conflict missing server copy: %v", stale)
	}
	call("DELETE", items+"/"+id, token, map[string]any{"version": 2}, 200)
	call("DELETE", items+"/"+id, token, map[string]any{"version": 2}, 409)
	afterDelete := call("GET", items, token, nil, 200)
	if len(afterDelete["items"].([]any)) != 0 {
		t.Fatalf("deleted item leaked into list: %v", afterDelete)
	}
	pull := call("GET", base+"/sync/pull?limit=2", token, nil, 200)
	events := pull["changes"].([]any)
	if len(events) != 2 || pull["next_cursor"] == nil {
		t.Fatalf("expected first page and cursor: %v", pull)
	}
	next := call("GET", base+"/sync/pull?cursor="+pull["next_cursor"].(string)+"&limit=100", token, nil, 200)
	if len(next["changes"].([]any)) == 0 {
		t.Fatalf("missing later changes: %v", next)
	}
	call("GET", base+"/sync/pull?cursor="+pull["next_cursor"].(string), otherToken, nil, 403)
	call("GET", "/v1/organizations/"+otherOrg+"/sync/pull?cursor="+pull["next_cursor"].(string), token, nil, 403)
	call("GET", base+"/sync/pull?cursor=not-valid", token, nil, 400)
	mutation := "00000000-aaaa-4bbb-8ccc-000000000002"
	newClient := "00000000-aaaa-4bbb-8ccc-000000000003"
	request := map[string]any{"mutations": []any{map[string]any{"mutation_id": mutation, "action": "create", "entity": "inspection_item", "inspection_id": inspection, "client_id": newClient, "title": "Fresh offline note", "notes": "offline text", "severity": "normal"}}}
	pushed := call("POST", base+"/sync/push", token, request, 200)
	replayed := call("POST", base+"/sync/push", token, request, 200)
	if fmt.Sprint(pushed["results"]) != fmt.Sprint(replayed["results"]) {
		t.Fatalf("replay changed result: %v vs %v", pushed, replayed)
	}
	sameID := map[string]any{"mutations": []any{map[string]any{"mutation_id": mutation, "action": "create", "entity": "inspection_item", "inspection_id": inspection, "client_id": newClient, "title": "Tampered payload"}}}
	call("POST", base+"/sync/push", token, sameID, 409)
	// Update via offline push; a second stale mutation must report 409 and a
	// server snapshot, rather than overwrite another inspector's changes.
	entry := pushed["results"].([]any)[0].(map[string]any)
	newID := entry["id"].(string)
	updateKey := "00000000-aaaa-4bbb-8ccc-000000000004"
	update := map[string]any{"mutations": []any{map[string]any{
		"mutation_id": updateKey, "action": "update", "entity": "inspection_item",
		"inspection_id": inspection, "id": newID, "version": 1, "status": "in_progress",
	}}}
	updated := call("POST", base+"/sync/push", token, update, 200)
	if updated["results"].([]any)[0].(map[string]any)["status"] != float64(200) {
		t.Fatalf("sync update rejected: %v", updated)
	}
	staleKey := "00000000-aaaa-4bbb-8ccc-000000000005"
	stalePush := map[string]any{"mutations": []any{map[string]any{
		"mutation_id": staleKey, "action": "update", "entity": "inspection_item",
		"inspection_id": inspection, "id": newID, "version": 1, "status": "resolved",
	}}}
	rejected := call("POST", base+"/sync/push", token, stalePush, 200)
	conflict := rejected["results"].([]any)[0].(map[string]any)
	if conflict["status"] != float64(409) || conflict["current"] == nil {
		t.Fatalf("missing conflict snapshot: %v", rejected)
	}
	// Client explicitly resolves by using the current server version.
	resolved := map[string]any{"mutations": []any{map[string]any{
		"mutation_id": "00000000-aaaa-4bbb-8ccc-000000000006", "action": "update",
		"entity": "inspection_item", "inspection_id": inspection, "id": newID,
		"version": 2, "status": "resolved",
	}}}
	synced := call("POST", base+"/sync/push", token, resolved, 200)
	if synced["results"].([]any)[0].(map[string]any)["status"] != float64(200) {
		t.Fatalf("resolution failed: %v", synced)
	}
	// The user may intentionally clear a field. Empty string and omitted notes
	// are different operations and must not silently leave stale local text.
	clearNotes := map[string]any{"mutations": []any{map[string]any{
		"mutation_id": "00000000-aaaa-4bbb-8ccc-000000000007",
		"action":      "update", "entity": "inspection_item",
		"inspection_id": inspection, "id": newID, "version": 3, "notes": "",
	}}}
	cleared := call("POST", base+"/sync/push", token, clearNotes, 200)
	if cleared["results"].([]any)[0].(map[string]any)["status"] != float64(200) {
		t.Fatalf("clear notes rejected: %v", cleared)
	}
	var savedNotes string
	if err = db.DB.QueryRow(ctx, `SELECT notes FROM inspection_items WHERE id=$1`, newID).Scan(&savedNotes); err != nil {
		t.Fatal(err)
	}
	if savedNotes != "" {
		t.Fatalf("offline notes were not cleared: %q", savedNotes)
	}

	call("POST", base+"/sync/push", otherToken, request, 403)
	// Cursor seek pagination for large lists must not produce duplicate rows.
	var secondProperty string
	if err = db.DB.QueryRow(ctx, `INSERT INTO properties(organization_id,name) VALUES($1,'Second property') RETURNING id`, org).Scan(&secondProperty); err != nil {
		t.Fatal(err)
	}
	page := call("GET", base+"/properties?limit=1", token, nil, 200)
	if page["next_cursor"] == nil {
		t.Fatalf("missing seek cursor: %v", page)
	}
	if page["has_more"] != true {
		t.Fatalf("expected second page: %v", page)
	}
	firstID := page["items"].([]any)[0].(map[string]any)["id"]
	follow := call("GET", base+"/properties?limit=1&cursor="+page["next_cursor"].(string), token, nil, 200)
	if len(follow["items"].([]any)) != 1 || follow["items"].([]any)[0].(map[string]any)["id"] == firstID {
		t.Fatalf("cursor missed or duplicated a property: %v %v", page, follow)
	}
	call("GET", base+"/inspections?limit=1&cursor="+page["next_cursor"].(string), token, nil, 400)
	// A malformed batch must not partially apply its first valid mutation.
	var uncommittedID = "00000000-aaaa-4bbb-8ccc-000000000008"
	malformed := map[string]any{"mutations": []any{
		map[string]any{"mutation_id": "00000000-aaaa-4bbb-8ccc-000000000009", "action": "create", "entity": "inspection_item",
			"inspection_id": inspection, "client_id": uncommittedID, "title": "Must not be inserted"},
		map[string]any{"mutation_id": "00000000-aaaa-4bbb-8ccc-000000000010", "action": "create", "entity": "inspection_item",
			"inspection_id": inspection, "client_id": "00000000-aaaa-4bbb-8ccc-000000000011", "title": ""},
	}}
	call("POST", base+"/sync/push", token, malformed, 400)
	var leaked int
	if err = db.DB.QueryRow(ctx, `SELECT count(*) FROM inspection_items WHERE organization_id=$1 AND client_id=$2`, org, uncommittedID).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("invalid batch partially applied %d mutations", leaked)
	}

}
