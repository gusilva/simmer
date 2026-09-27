package device

import (
	"net"
	"testing"
)

func TestIsPrivateLANAddr(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.25", true},
		{"192.168.0.1", true},
		{"10.0.0.5", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"100.67.208.78", false}, // Tailscale/CGNAT range
		{"8.8.8.8", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip).To4()
			if ip == nil {
				t.Fatalf("failed to parse %q", tt.ip)
			}
			if got := isPrivateLANAddr(ip); got != tt.want {
				t.Errorf("isPrivateLANAddr(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
