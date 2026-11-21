package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRateLimitIP(t *testing.T) {
	secret := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		name      string
		config    string
		proof     []string
		forwarded []string
		want      string
	}{
		{"disabled", "", []string{secret}, []string{"192.0.2.5"}, "172.18.0.7"},
		{"forged", secret, []string{"wrong"}, []string{"192.0.2.5"}, "172.18.0.7"},
		{"no proof", secret, nil, []string{"192.0.2.5"}, "172.18.0.7"},
		{"generic XFF only", secret, []string{secret}, nil, "172.18.0.7"},
		{"valid IPv4", secret, []string{secret}, []string{"192.0.2.5"}, "192.0.2.5"},
		{"mapped IPv4", secret, []string{secret}, []string{"::ffff:192.0.2.5"}, "192.0.2.5"},
		{"IPv6 subnet", secret, []string{secret}, []string{"2001:db8:1:2::123"}, "2001:db8:1:2::"},
		{"IPv6 same subnet", secret, []string{secret}, []string{"2001:db8:1:2::456"}, "2001:db8:1:2::"},
		{"chain", secret, []string{secret}, []string{"192.0.2.5, 192.0.2.6"}, "172.18.0.7"},
		{"duplicate IP", secret, []string{secret}, []string{"192.0.2.5", "192.0.2.6"}, "172.18.0.7"},
		{"duplicate proof", secret, []string{secret, secret}, []string{"192.0.2.5"}, "172.18.0.7"},
		{"invalid IP", secret, []string{secret}, []string{"999.0.0.1"}, "172.18.0.7"},
		{"port", secret, []string{secret}, []string{"192.0.2.5:1234"}, "172.18.0.7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			if err := s.ConfigureTrustedBFF(tc.config); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/", nil)
			r.RemoteAddr = "172.18.0.7:12345"
			r.Header.Set("X-Forwarded-For", "198.51.100.88")
			r.Header.Set("X-Real-IP", "198.51.100.99")
			for _, proof := range tc.proof {
				r.Header.Add(proxySecretHeader, proof)
			}
			for _, ip := range tc.forwarded {
				r.Header.Add(clientIPHeader, ip)
			}
			if got := s.clientRateLimitIP(r); got != tc.want {
				t.Fatalf("IP: got %q, want %q", got, tc.want)
			}
		})
	}
	if err := (&Server{}).ConfigureTrustedBFF("short"); err == nil {
		t.Fatal("short proxy secret accepted")
	}
}
