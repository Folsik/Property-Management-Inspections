package api

import "testing"

func TestPINPolicy(t *testing.T) {
	tests := []struct {
		pin   string
		valid bool
	}{
		{"123456", false},
		{"654321", false},
		{"111111", false},
		{"918245", true},
		{"12345", false},
		{"abc123", false},
	}
	for _, tc := range tests {
		if got := pinOK(tc.pin); got != tc.valid {
			t.Errorf("pinOK(%q)=%t, want %t", tc.pin, got, tc.valid)
		}
	}
}
