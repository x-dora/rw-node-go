package xray

import (
	"net"
	"testing"
)

func TestRoutingCIDR(t *testing.T) {
	ip, prefix := routingCIDR(net.ParseIP("203.0.113.7"))
	if string(ip) != string([]byte{203, 0, 113, 7}) || prefix != 32 {
		t.Fatalf("IPv4 cidr = %v/%d", ip, prefix)
	}

	ip, prefix = routingCIDR(net.ParseIP("2001:db8::1"))
	if len(ip) != net.IPv6len || prefix != 128 {
		t.Fatalf("IPv6 cidr = %v/%d", ip, prefix)
	}
}
