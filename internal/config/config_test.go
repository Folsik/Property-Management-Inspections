package config

import "testing"

func TestEnvironmentFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{
		{"", "production"},
		{"production", "production"},
		{"staging", "production"},
		{"Development", "production"},
		{"development", "development"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.value)
			if got := Load().Env; got != tc.want {
				t.Fatalf("APP_ENV=%q: got %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
