package system

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
)

var ErrConntrackUnavailable = errors.New("conntrack unavailable")

type Conntrack struct{}

func IsConntrackUnavailable(err error) bool {
	return errors.Is(err, ErrConntrackUnavailable)
}

func parseConntrackIP(value string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.IsValid() {
		return netip.Addr{}, fmt.Errorf("invalid ip %q", value)
	}
	return addr.Unmap(), nil
}

func (c Conntrack) DropIP(ctx context.Context, ip string) error {
	return c.dropIP(ctx, ip)
}
