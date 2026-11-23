package api

import (
	"io"
	"log/slog"
	"testing"
)

func TestRefreshResponseEncryptionKeyIsolation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	first := New(nil, nil, logger, "development")
	second := New(nil, nil, logger, "development")
	third := New(nil, nil, logger, "development")
	first.ConfigureRefreshReplayKey("deployment-shared-signing-key")
	second.ConfigureRefreshReplayKey("deployment-shared-signing-key")
	third.ConfigureRefreshReplayKey("different-signing-key")
	secret := "example-token-must-not-appear-in-plaintext"
	encrypted, err := first.encryptRefreshResponse(map[string]any{"refresh_token": secret, "session_id": "example"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := second.decryptRefreshResponse(encrypted)
	if err != nil || value["refresh_token"] != secret {
		t.Fatalf("same deployment cannot restore response: %v", err)
	}
	if _, err = third.decryptRefreshResponse(encrypted); err == nil {
		t.Fatal("unrelated signing keys must not decrypt refresh credentials")
	}
}
