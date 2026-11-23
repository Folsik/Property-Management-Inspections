package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDContextAndSanitizedAccessLog(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/public/reports/{token}", func(w http.ResponseWriter, r *http.Request) {
		if requestIDFromContext(r.Context()) == "" {
			t.Error("request ID missing from context")
		}
		w.WriteHeader(http.StatusNotFound)
	})
	h := requestID(logger, mux)
	const secret = "this-secret-token-must-not-be-logged"
	req := httptest.NewRequest("GET", "/v1/public/reports/"+secret, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 404 || len(w.Header().Get("X-Request-ID")) != 24 {
		t.Fatalf("unexpected response status=%d request_id=%q", w.Code, w.Header().Get("X-Request-ID"))
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatal("access log contains untrusted token path")
	}
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &event); err != nil {
		t.Fatal(err)
	}
	if event["request_id"] != w.Header().Get("X-Request-ID") {
		t.Fatal("request ID was not propagated to access log")
	}
	if event["route"] != "GET /v1/public/reports/{token}" {
		t.Fatalf("unsafe or incorrect route: %#v", event["route"])
	}
	if event["status"] != float64(404) {
		t.Fatalf("unexpected access status %#v", event["status"])
	}
}
