package api

import (
	"strings"
	"testing"
)

func TestPasswordPolicy(t *testing.T) {
	good := []string{
		"four green lanterns beside the river",
		"a quiet blue ocean under bright stars",
		"сложная длинная фраза для входа",
		"correct horse battery staple with a twist",
		strings.Repeat("💧", 64),
		strings.Repeat("a", 70) + " differing words at the end",
	}
	for _, p := range good {
		if err := validatePassword(p, "+14155552671", "person@example.test"); err != nil {
			t.Errorf("passphrase %q rejected: %v", p, err)
		}
	}
	bad := []string{"short phrase", "passwordpassword", "correct horse battery staple", "123456789012345", "qwertyqwertyqwerty", "person@example.test", strings.Repeat("a", 129)}
	for _, p := range bad {
		if err := validatePassword(p, "+14155552671", "person@example.test"); err == nil {
			t.Errorf("known/short password accepted: %q", p)
		}
	}
}
func TestPasswordHashAndLegacyVerification(t *testing.T) {
	p := "long entirely lowercase secret with emoji 🔐"
	h, err := hashPassword(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("must use argon2id, got %q", h)
	}
	if !verifyPassword(h, p) || verifyPassword(h, p+"x") {
		t.Fatal("argon2 comparison failed")
	}
	if !verifyPassword(h, strings.Replace(p, "🔐", "🔐", 1)) {
		t.Fatal("unicode password failed")
	}
	legacy := "$2a$10$ROOskaGYq9NOqJpmrdi1bOWzuUadTBCj8Y.4.fJFBJKVnYZpGBkt."
	if verifyPassword(legacy, "incorrect") {
		t.Fatal("legacy verifier accepted incorrect password")
	}
}
