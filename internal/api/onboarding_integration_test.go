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

func TestGuidedOnboardingAndInvitations(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	database, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	tm, err := auth.NewManager("onboarding-test", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.New(database.DB, tm, slog.New(slog.NewTextHandler(io.Discard, nil)), "development").Handler())
	defer srv.Close()
	call := func(method, path, token string, request any, expected int) map[string]any {
		t.Helper()
		var input io.Reader
		if request != nil {
			data, _ := json.Marshal(request)
			input = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(ctx, method, srv.URL+path, input)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var output map[string]any
		if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != expected {
			t.Fatalf("%s %s: expected %d got %d: %#v", method, path, expected, response.StatusCode, output)
		}
		return output
	}
	createUser := func(phone string) string {
		t.Helper()
		otp := call("POST", "/v1/auth/otp/request", "", map[string]string{"phone": phone}, 202)
		access := call("POST", "/v1/auth/otp/verify", "", map[string]string{"phone": phone, "code": otp["dev_code"].(string)}, 200)
		return access["access_token"].(string)
	}
	stamp := time.Now().UnixNano() % 100000000
	ownerPhone := fmt.Sprintf("+1555%08d", stamp)
	workerPhone := fmt.Sprintf("+1666%08d", stamp)
	owner := createUser(ownerPhone)
	call("GET", "/v1/onboarding", owner, nil, 200)
	call("POST", "/v1/onboarding/complete", owner, map[string]string{}, 409)
	call("POST", "/v1/auth/password", owner, map[string]string{"password": "Cobalt!Harbor7May"}, 200)
	call("POST", "/v1/auth/password", owner, map[string]string{"password": "Welcome!Rivers32Lake"}, 409)
	call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": ownerPhone, "password": "wrong"}, 401)
	call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": ownerPhone, "password": "Cobalt!Harbor7May"}, 200)
	// Enrolled password must not be bypassed using a new phone OTP.
	secondOTP := call("POST", "/v1/auth/otp/request", "", map[string]string{"phone": ownerPhone}, 202)
	call("POST", "/v1/auth/otp/verify", "", map[string]string{"phone": ownerPhone, "code": secondOTP["dev_code"].(string)}, 409)
	call("PATCH", "/v1/me", owner, map[string]string{"full_name": "Property Owner"}, 200)
	email := fmt.Sprintf("onboarding-%d@example.test", stamp)
	challenge := call("POST", "/v1/email/request", owner, map[string]string{"email": email}, 202)
	if challenge["dev_code"] == nil {
		t.Fatal("development email challenge missing")
	}
	call("POST", "/v1/email/verify", owner, map[string]string{"email": email, "code": "000000"}, 401)
	call("POST", "/v1/email/verify", owner, map[string]string{"email": email, "code": challenge["dev_code"].(string)}, 200)
	call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": email, "password": "Cobalt!Harbor7May"}, 200)
	call("POST", "/v1/email/verify", owner, map[string]string{"email": email, "code": challenge["dev_code"].(string)}, 401)
	org := call("POST", "/v1/organizations", owner, map[string]string{"name": "Owner portfolio", "kind": "individual", "country": "GB"}, 201)["id"].(string)
	call("POST", "/v1/onboarding/complete", owner, map[string]string{}, 409)
	prop := call("POST", "/v1/organizations/"+org+"/properties", owner, map[string]any{"name": "Apartment", "address": map[string]string{"street": "1 Main Street", "city": "London", "country": "GB"}}, 201)
	if prop["id"] == nil {
		t.Fatal("property missing")
	}
	call("POST", "/v1/onboarding/complete", owner, map[string]string{}, 200)
	status := call("GET", "/v1/onboarding", owner, nil, 200)
	if status["completed"] != true {
		t.Fatalf("not completed: %#v", status)
	}

	worker := createUser(workerPhone)
	workerEmail := fmt.Sprintf("inspector-%d@example.test", stamp)
	invite := call("POST", "/v1/organizations/"+org+"/invitations", owner, map[string]string{"email": workerEmail, "role": "inspector"}, 201)
	inviteToken, ok := invite["token"].(string)
	if !ok || inviteToken == "" {
		t.Fatal("missing one-time invite token")
	}
	call("GET", "/v1/organizations/"+org+"/invitations", owner, nil, 200)
	call("GET", "/v1/invitations/"+inviteToken, "", nil, 200)
	// Resending an invitation must revoke the previous bearer token.
	renewed := call("POST", "/v1/organizations/"+org+"/invitations", owner, map[string]string{"email": workerEmail, "role": "inspector"}, 201)
	call("GET", "/v1/invitations/"+inviteToken, "", nil, 404)
	inviteToken = renewed["token"].(string)
	call("GET", "/v1/invitations/"+inviteToken, "", nil, 200)
	call("POST", "/v1/invitations/"+inviteToken+"/accept", worker, map[string]string{}, 403)
	emailCode := call("POST", "/v1/email/request", worker, map[string]string{"email": workerEmail}, 202)["dev_code"].(string)
	call("POST", "/v1/email/verify", worker, map[string]string{"email": workerEmail, "code": emailCode}, 200)
	call("POST", "/v1/auth/password", worker, map[string]string{"password": "Welcome!Rivers32Lake"}, 200)
	call("PATCH", "/v1/me", worker, map[string]string{"full_name": "Invited Inspector"}, 200)
	call("POST", "/v1/invitations/"+inviteToken+"/accept", worker, map[string]string{}, 200)
	call("POST", "/v1/invitations/"+inviteToken+"/accept", worker, map[string]string{}, 409)
	call("GET", "/v1/invitations/"+inviteToken, "", nil, 404)
	call("POST", "/v1/onboarding/complete", worker, map[string]string{}, 200)
	call("POST", "/v1/organizations/"+org+"/invitations", worker, map[string]string{"email": "outsider@example.test", "role": "manager"}, 403)
	managerInvite := call("POST", "/v1/organizations/"+org+"/invitations", owner, map[string]string{"email": "manager@example.test", "role": "manager"}, 201)
	managerID := managerInvite["id"].(string)
	call("DELETE", "/v1/organizations/"+org+"/invitations/"+managerID, owner, nil, 200)
	call("GET", "/v1/invitations/"+managerInvite["token"].(string), "", nil, 404)
	call("GET", "/v1/organizations/"+org+"/invitations", worker, nil, 403)
	call("POST", "/v1/auth/password/change", worker, map[string]string{"current_password": "incorrect", "new_password": "New#SeasideHarbor84"}, 401)
	call("POST", "/v1/auth/password/change", worker, map[string]string{"current_password": "Welcome!Rivers32Lake", "new_password": "New#SeasideHarbor84"}, 200)
	call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": workerEmail, "password": "Welcome!Rivers32Lake"}, 401)
	call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": workerEmail, "password": "New#SeasideHarbor84"}, 200)
	// Recovery requires both an email token and a phone code and never grants a session.
	resetEmail := call("POST", "/v1/auth/recovery/request", "", map[string]string{"email": workerEmail}, 202)
	if resetEmail["dev_token"] == nil || resetEmail["dev_code"] == nil {
		t.Fatal("development recovery challenge missing")
	}
	call("POST", "/v1/auth/recovery/request", "", map[string]string{"email": "nonexistent-" + workerEmail}, 202)
	call("POST", "/v1/auth/recovery/complete", "", map[string]string{"token": resetEmail["dev_token"].(string), "code": "000000", "password": "an entirely new long passphrase"}, 401)
	call("POST", "/v1/auth/recovery/complete", "", map[string]string{"token": resetEmail["dev_token"].(string), "code": resetEmail["dev_code"].(string), "password": "a fresh strong passphrase from memory"}, 200)
	call("POST", "/v1/auth/recovery/complete", "", map[string]string{"token": resetEmail["dev_token"].(string), "code": resetEmail["dev_code"].(string), "password": "yet another entirely new passphrase"}, 401)
	call("GET", "/v1/me", worker, nil, 401)
	call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": workerEmail, "password": "a fresh strong passphrase from memory"}, 200)
	// Failed attempts incur bounded backoff without allowing an attacker to lock out the legitimate owner.
	for i := 0; i < 5; i++ {
		call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": workerEmail, "password": "wrong-password"}, 401)
	}
	call("POST", "/v1/auth/password/login", "", map[string]string{"identifier": workerEmail, "password": "a fresh strong passphrase from memory"}, 200)
}
