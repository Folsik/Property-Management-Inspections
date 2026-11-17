package api

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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Folsik/Property-Management-Inspections/internal/auth"
	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func TestMediaSignatures(t *testing.T) {
	samples := []struct {
		mime  string
		body  []byte
		valid bool
	}{
		{"image/jpeg", []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0}, true},
		{"image/jpeg", []byte("<html>fake jpeg</html>"), false},
		{"image/png", append([]byte{137, 80, 78, 71, 13, 10, 26, 10}, make([]byte, 12)...), true},
		{"image/heic", []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c', 0, 0, 0, 0}, true},
		{"audio/mp4", []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'M', '4', 'A', ' '}, true},
		{"audio/mpeg", append([]byte("ID3"), make([]byte, 12)...), true},
		{"audio/mpeg", []byte("<xml>no audio</xml>"), false},
	}
	for _, x := range samples {
		if got := validMediaSignature(x.mime, x.body); got != x.valid {
			t.Errorf("%s signature=%v expected %v", x.mime, got, x.valid)
		}
	}
}
func TestPresignReadAndStorageRequest(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	for _, method := range []string{"GET", "HEAD", "DELETE"} {
		u, err := presignObjectURL("https://storage.example.test", "media", "auto", "access", "secret", "organizations/uuid/inspections/uuid/abcd", method, "", now, 300)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(u, "X-Amz-Signature=") || !strings.Contains(u, "X-Amz-SignedHeaders=host") {
			t.Fatalf("unsigned %s: %s", method, u)
		}
	}
}

func TestMediaLifecycleAndTenantIsolation(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 85*time.Second)
	defer cancel()
	database, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	type stored struct {
		body []byte
		mime string
	}
	objects := map[string]stored{}
	fake := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("X-Amz-Signature") == "" {
			w.WriteHeader(403)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/media/")
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "PUT":
			body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			objects[key] = stored{body, r.Header.Get("Content-Type")}
			w.WriteHeader(200)
		case "HEAD":
			v, ok := objects[key]
			if !ok {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Type", v.mime)
			w.Header().Set("Content-Length", strconv.Itoa(len(v.body)))
			w.WriteHeader(200)
		case "GET":
			v, ok := objects[key]
			if !ok {
				w.WriteHeader(404)
				return
			}
			if r.Header.Get("Range") != "" {
				w.WriteHeader(206)
				w.Write(v.body[:min(len(v.body), 64)])
				return
			}
			w.Header().Set("Content-Type", v.mime)
			w.Write(v.body)
		case "DELETE":
			delete(objects, key)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}))
	defer fake.Close()
	t.Setenv("S3_ENDPOINT", fake.URL)
	t.Setenv("S3_BUCKET", "media")
	t.Setenv("S3_REGION", "auto")
	t.Setenv("S3_ACCESS_KEY_ID", "access")
	t.Setenv("S3_SECRET_ACCESS_KEY", "secret")
	manager, err := auth.NewManager("media-tests", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service := New(database.DB, manager, slog.New(slog.NewTextHandler(io.Discard, nil)), "development")
	service.storageHTTPClient = fake.Client()
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	var owner, outsider, org, otherOrg, inspection string
	stamp := time.Now().UnixNano()
	for i, p := range []*string{&owner, &outsider} {
		err = database.DB.QueryRow(ctx, `INSERT INTO users(phone) VALUES($1) RETURNING id`, fmt.Sprintf("+18%013d", stamp%1000000000000+int64(i))).Scan(p)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, p := range []*string{&org, &otherOrg} {
		err = database.DB.QueryRow(ctx, `INSERT INTO organizations(name,created_by) VALUES($1,$2) RETURNING id`, fmt.Sprintf("Media test %d", i), owner).Scan(p)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = database.DB.Exec(ctx, `INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,'owner'),($3,$2,'owner')`, org, owner, otherOrg)
	if err != nil {
		t.Fatal(err)
	}
	var property string
	err = database.DB.QueryRow(ctx, `INSERT INTO properties(organization_id,name) VALUES($1,'Unit') RETURNING id`, org).Scan(&property)
	if err != nil {
		t.Fatal(err)
	}
	err = database.DB.QueryRow(ctx, `INSERT INTO inspections(organization_id,property_id,created_by,inspection_type) VALUES($1,$2,$3,'turnover') RETURNING id`, org, property, owner).Scan(&inspection)
	if err != nil {
		t.Fatal(err)
	}
	ownerJWT, _, err := manager.Issue(owner)
	if err != nil {
		t.Fatal(err)
	}
	otherJWT, _, err := manager.Issue(outsider)
	if err != nil {
		t.Fatal(err)
	}
	base := "/v1/organizations/" + org + "/inspections/" + inspection + "/media"
	request := func(method, path, token string, body string, want int) map[string]any {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		if res.StatusCode != want {
			t.Fatalf("%s %s wanted %d, got %d: %v", method, path, want, res.StatusCode, out)
		}
		return out
	}
	ticket := request("POST", base+"/presign", ownerJWT, `{"mime_type":"image/jpeg","size_bytes":6}`, 201)
	id := ticket["id"].(string)
	request("POST", base+"/"+id+"/confirm", otherJWT, "{}", 403)
	request("POST", base+"/"+id+"/confirm", ownerJWT, "{}", 409)
	uploadURL := ticket["upload_url"].(string)
	put, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xe0, 0, 0}))
	if err != nil {
		t.Fatal(err)
	}
	put.Header.Set("Content-Type", "image/jpeg")
	res, err := fake.Client().Do(put)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("storage PUT %d", res.StatusCode)
	}
	request("POST", base+"/"+id+"/confirm", ownerJWT, "{}", 200)
	request("POST", base+"/"+id+"/confirm", ownerJWT, "{}", 200)
	read := request("GET", base+"/"+id+"/download", ownerJWT, "", 200)
	if read["download_url"] == nil {
		t.Fatal("missing signed download")
	}
	request("GET", base+"/"+id+"/download", otherJWT, "", 403)
	request("GET", "/v1/organizations/"+otherOrg+"/inspections/"+inspection+"/media/"+id+"/download", ownerJWT, "", 404)
	request("DELETE", base+"/"+id, otherJWT, "", 403)
	request("DELETE", base+"/"+id, ownerJWT, "", 200)
	request("GET", base+"/"+id+"/download", ownerJWT, "", 404)
	// Pending rows that outlive the expiry must be garbage collected, not served.
	stale := request("POST", base+"/presign", ownerJWT, `{"mime_type":"image/jpeg","size_bytes":6}`, 201)
	staleID := stale["id"].(string)
	_, err = database.DB.Exec(ctx, `UPDATE media_attachments SET created_at=now()-interval '2 hours' WHERE id=$1`, staleID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CleanupStaleMedia(ctx, 20); err != nil {
		t.Fatal(err)
	}
	request("POST", base+"/"+staleID+"/confirm", ownerJWT, "{}", 409)
}
