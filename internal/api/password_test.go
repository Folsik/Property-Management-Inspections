package api

import "testing"

func TestPasswordPolicy(t *testing.T) {
	good := []string{"Cobalt!Harbor7May", "Welcome!Rivers32Lake", "Zebra#FrostCloud9"}
	for _, p := range good {
		if err := validatePassword(p, "+14155552671", "person@example.test"); err != nil {
			t.Errorf("strong password rejected %q: %v", p, err)
		}
	}
	bad := []string{"Short!9A", "alllowercase!123", "ALLUPPERCASE!123", "NoSymbols12345", "aaaaaaaaaaaaA1!", "Password123!", "Qwerty123!XyZ", "1234567890Ab!", "Person123!Welcome", "Zebra#FrostCloud9" + string(make([]byte, 75))}
	for _, p := range bad {
		if err := validatePassword(p, "+14155552671", "person@example.test"); err == nil {
			t.Errorf("weak password accepted %q", p)
		}
	}
}
