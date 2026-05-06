package xray

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common/serial"
	featurerouting "github.com/xtls/xray-core/features/routing"
)

type RoutingRule struct {
	InboundTags []string
	OutboundTag string
}

type routerWithRules interface {
	featurerouting.Router
	AddRule(msg *serial.TypedMessage, shouldAppend bool) error
	RemoveRule(tag string) error
}

func (c embeddedRoutingClient) AddSourceIPRule(ctx context.Context, ruleTag string, sourceIP string, outboundTag string) error {
	routerFeature, err := c.router()
	if err != nil {
		return err
	}
	ip := net.ParseIP(sourceIP)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", sourceIP)
	}

	ipBytes, prefix := routingCIDR(ip)
	config := &router.Config{
		Rule: []*router.RoutingRule{
			{
				RuleTag: ruleTag,
				TargetTag: &router.RoutingRule_Tag{
					Tag: outboundTag,
				},
				SourceGeoip: []*router.GeoIP{
					{
						Cidr: []*router.CIDR{
							{Ip: ipBytes, Prefix: prefix},
						},
					},
				},
			},
		},
	}
	if err := routerFeature.AddRule(serial.ToTypedMessage(config), true); err != nil {
		return fmt.Errorf("add xray routing rule %q: %w", ruleTag, err)
	}
	return nil
}

func (c embeddedRoutingClient) RemoveRule(ctx context.Context, ruleTag string) error {
	routerFeature, err := c.router()
	if err != nil {
		return err
	}
	if err := routerFeature.RemoveRule(ruleTag); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "empty tag") {
			return nil
		}
		return fmt.Errorf("remove xray routing rule %q: %w", ruleTag, err)
	}
	return nil
}

func (c embeddedRoutingClient) router() (routerWithRules, error) {
	instance := c.core.Instance()
	if instance == nil {
		return nil, fmt.Errorf("xray is not running")
	}
	feature := instance.GetFeature(featurerouting.RouterType())
	if feature == nil {
		return nil, fmt.Errorf("xray routing feature is unavailable")
	}
	routerFeature, ok := feature.(routerWithRules)
	if !ok {
		return nil, fmt.Errorf("xray routing feature does not support dynamic rules")
	}
	return routerFeature, nil
}

func routingCIDR(ip net.IP) ([]byte, uint32) {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4, 32
	}
	return ip.To16(), 128
}
