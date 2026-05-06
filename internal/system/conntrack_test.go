package system

import (
	"context"
	"testing"
)

func TestParseConntrackIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{name: "ipv4", ip: "203.0.113.1", want: "203.0.113.1"},
		{name: "ipv6", ip: "2001:db8::1", want: "2001:db8::1"},
		{name: "mapped ipv4", ip: "::ffff:203.0.113.1", want: "203.0.113.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConntrackIP(tt.ip)
			if err != nil {
				t.Fatalf("parseConntrackIP returned error: %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("addr = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

func TestParseConntrackIPRejectsInvalidIP(t *testing.T) {
	if _, err := parseConntrackIP("not-an-ip"); err == nil {
		t.Fatalf("parseConntrackIP returned nil error")
	}
}

func TestConntrackDropIPRejectsInvalidIPBeforeUnavailableFallback(t *testing.T) {
	err := Conntrack{}.DropIP(context.Background(), "not-an-ip")
	if err == nil {
		t.Fatalf("DropIP returned nil error")
	}
	if IsConntrackUnavailable(err) {
		t.Fatalf("invalid IP was reported as conntrack unavailable: %v", err)
	}
}

func TestConntrackDropIPReportsUnavailableWithoutSystemCapability(t *testing.T) {
	err := Conntrack{}.DropIP(context.Background(), "203.0.113.1")
	if err == nil {
		if HasNetAdmin() {
			t.Skip("host has CAP_NET_ADMIN and conntrack deletion returned nil")
		}
		t.Fatalf("DropIP returned nil error without CAP_NET_ADMIN")
	}
	if !IsConntrackUnavailable(err) {
		t.Fatalf("DropIP error = %v, want conntrack unavailable", err)
	}
}
