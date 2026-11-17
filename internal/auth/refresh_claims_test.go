package auth

import (
	"testing"
	"time"
)

func TestSessionClaimsAreSignedAndValidated(t *testing.T) {
	m, err := NewManager("session-claims", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := m.IssueWithSession("a-user", 3, "11111111-1111-4111-8111-111111111111", 5)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "11111111-1111-4111-8111-111111111111" || got.SessionGeneration != 5 || got.AuthEpoch != 3 {
		t.Fatalf("wrong session scope: %#v", got)
	}
}
