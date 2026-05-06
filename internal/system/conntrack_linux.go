//go:build linux

package system

import (
	"context"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func (c Conntrack) dropIP(ctx context.Context, value string) error {
	addr, err := parseConntrackIP(value)
	if err != nil {
		return err
	}
	if !HasNetAdmin() {
		return fmt.Errorf("%w: CAP_NET_ADMIN is unavailable", ErrConntrackUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	ip := net.ParseIP(addr.String())
	if ip == nil {
		return fmt.Errorf("invalid ip %q", value)
	}

	family := netlink.InetFamily(unix.AF_INET)
	if addr.Is6() {
		family = netlink.InetFamily(unix.AF_INET6)
	}

	filters, err := conntrackIPFilters(ip)
	if err != nil {
		return err
	}
	_, err = netlink.ConntrackDeleteFilters(netlink.ConntrackTable, family, filters...)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConntrackUnavailable, err)
	}
	return nil
}

func conntrackIPFilters(ip net.IP) ([]netlink.CustomConntrackFilter, error) {
	filterTypes := []netlink.ConntrackFilterType{
		netlink.ConntrackOrigSrcIP,
		netlink.ConntrackOrigDstIP,
		netlink.ConntrackReplySrcIP,
		netlink.ConntrackReplyDstIP,
	}
	filters := make([]netlink.CustomConntrackFilter, 0, len(filterTypes))
	for _, filterType := range filterTypes {
		filter := &netlink.ConntrackFilter{}
		if err := filter.AddIP(filterType, ip); err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	return filters, nil
}
