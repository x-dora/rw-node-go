package xray

import (
	"context"
	"fmt"
	"sort"
	"strings"

	statscommand "github.com/xtls/xray-core/app/stats/command"
)

const (
	StatsUserUplinkFormat       = "user>>>%s>>>traffic>>>uplink"
	StatsUserDownlinkFormat     = "user>>>%s>>>traffic>>>downlink"
	StatsUserOnlineFormat       = "user>>>%s>>>online"
	StatsInboundUplinkFormat    = "inbound>>>%s>>>traffic>>>uplink"
	StatsInboundDownlinkFormat  = "inbound>>>%s>>>traffic>>>downlink"
	StatsOutboundUplinkFormat   = "outbound>>>%s>>>traffic>>>uplink"
	StatsOutboundDownlinkFormat = "outbound>>>%s>>>traffic>>>downlink"
)

const (
	statsUserPattern     = "user>>>"
	statsInboundPattern  = "inbound>>>"
	statsOutboundPattern = "outbound>>>"
)

type SysStats struct {
	NumGoroutine uint32
	NumGC        uint32
	Alloc        uint64
	TotalAlloc   uint64
	Sys          uint64
	Mallocs      uint64
	Frees        uint64
	LiveObjects  uint64
	PauseTotalNs uint64
	Uptime       uint32
}

type UserTrafficStats struct {
	Username string
	Downlink int64
	Uplink   int64
}

type UserIPList struct {
	Username string
	IPs      []IPLastSeen
}

type IPLastSeen struct {
	IP       string
	LastSeen int64
}

type InboundTrafficStats struct {
	Inbound  string
	Downlink int64
	Uplink   int64
}

type OutboundTrafficStats struct {
	Outbound string
	Downlink int64
	Uplink   int64
}

func (c *statsClient) SysStats(ctx context.Context) (SysStats, error) {
	response, err := c.raw.GetSysStats(ctx, &statscommand.SysStatsRequest{})
	if err != nil {
		return SysStats{}, fmt.Errorf("xray stats get system stats: %w", err)
	}
	return SysStats{
		NumGoroutine: response.GetNumGoroutine(),
		NumGC:        response.GetNumGC(),
		Alloc:        response.GetAlloc(),
		TotalAlloc:   response.GetTotalAlloc(),
		Sys:          response.GetSys(),
		Mallocs:      response.GetMallocs(),
		Frees:        response.GetFrees(),
		LiveObjects:  response.GetLiveObjects(),
		PauseTotalNs: response.GetPauseTotalNs(),
		Uptime:       response.GetUptime(),
	}, nil
}

func (c *statsClient) UsersStats(ctx context.Context, reset bool) ([]UserTrafficStats, error) {
	response, err := c.query(ctx, statsUserPattern, reset)
	if err != nil {
		return nil, err
	}

	byUser := map[string]*UserTrafficStats{}
	for _, stat := range response.GetStat() {
		username, direction, ok := parseTrafficStatName(stat.GetName(), "user")
		if !ok {
			continue
		}
		item := byUser[username]
		if item == nil {
			item = &UserTrafficStats{Username: username}
			byUser[username] = item
		}
		applyDirection(direction, stat.GetValue(), &item.Uplink, &item.Downlink)
	}

	users := make([]UserTrafficStats, 0, len(byUser))
	for _, item := range byUser {
		if item.Uplink == 0 && item.Downlink == 0 {
			continue
		}
		users = append(users, *item)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})
	return users, nil
}

func (c *statsClient) UserOnlineStatus(ctx context.Context, username string) (bool, error) {
	response, err := c.raw.GetStatsOnline(ctx, &statscommand.GetStatsRequest{
		Name: fmt.Sprintf(StatsUserOnlineFormat, username),
	})
	if err != nil {
		return false, fmt.Errorf("xray stats get online status for user %q: %w", username, err)
	}
	stat := response.GetStat()
	return stat != nil && stat.GetValue() > 0, nil
}

func (c *statsClient) UserIPList(ctx context.Context, username string, reset bool) ([]IPLastSeen, error) {
	return c.onlineIPList(ctx, username, reset)
}

func (c *statsClient) UsersIPList(ctx context.Context, reset bool) ([]UserIPList, error) {
	response, err := c.raw.GetAllOnlineUsers(ctx, &statscommand.GetAllOnlineUsersRequest{})
	if err != nil {
		return nil, fmt.Errorf("xray stats get all online users: %w", err)
	}

	seen := map[string]struct{}{}
	users := make([]string, 0, len(response.GetUsers()))
	for _, raw := range response.GetUsers() {
		username, ok := parseOnlineStatName(raw)
		if !ok {
			continue
		}
		if _, exists := seen[username]; exists {
			continue
		}
		seen[username] = struct{}{}
		users = append(users, username)
	}
	sort.Strings(users)

	output := make([]UserIPList, 0, len(users))
	for _, username := range users {
		ips, err := c.onlineIPList(ctx, username, reset)
		if err != nil {
			continue
		}
		if len(ips) == 0 {
			continue
		}
		output = append(output, UserIPList{Username: username, IPs: ips})
	}
	return output, nil
}

func (c *statsClient) InboundStats(ctx context.Context, tag string, reset bool) (InboundTrafficStats, error) {
	uplink, err := c.getValue(ctx, fmt.Sprintf(StatsInboundUplinkFormat, tag), reset)
	if err != nil {
		return InboundTrafficStats{}, err
	}
	downlink, err := c.getValue(ctx, fmt.Sprintf(StatsInboundDownlinkFormat, tag), reset)
	if err != nil {
		return InboundTrafficStats{}, err
	}
	return InboundTrafficStats{Inbound: tag, Uplink: uplink, Downlink: downlink}, nil
}

func (c *statsClient) OutboundStats(ctx context.Context, tag string, reset bool) (OutboundTrafficStats, error) {
	uplink, err := c.getValue(ctx, fmt.Sprintf(StatsOutboundUplinkFormat, tag), reset)
	if err != nil {
		return OutboundTrafficStats{}, err
	}
	downlink, err := c.getValue(ctx, fmt.Sprintf(StatsOutboundDownlinkFormat, tag), reset)
	if err != nil {
		return OutboundTrafficStats{}, err
	}
	return OutboundTrafficStats{Outbound: tag, Uplink: uplink, Downlink: downlink}, nil
}

func (c *statsClient) AllInboundStats(ctx context.Context, reset bool) ([]InboundTrafficStats, error) {
	response, err := c.query(ctx, statsInboundPattern, reset)
	if err != nil {
		return nil, err
	}

	byTag := map[string]*InboundTrafficStats{}
	for _, stat := range response.GetStat() {
		tag, direction, ok := parseTrafficStatName(stat.GetName(), "inbound")
		if !ok {
			continue
		}
		item := byTag[tag]
		if item == nil {
			item = &InboundTrafficStats{Inbound: tag}
			byTag[tag] = item
		}
		applyDirection(direction, stat.GetValue(), &item.Uplink, &item.Downlink)
	}

	inbounds := make([]InboundTrafficStats, 0, len(byTag))
	for _, item := range byTag {
		inbounds = append(inbounds, *item)
	}
	sort.Slice(inbounds, func(i, j int) bool {
		return inbounds[i].Inbound < inbounds[j].Inbound
	})
	return inbounds, nil
}

func (c *statsClient) AllOutboundStats(ctx context.Context, reset bool) ([]OutboundTrafficStats, error) {
	response, err := c.query(ctx, statsOutboundPattern, reset)
	if err != nil {
		return nil, err
	}

	byTag := map[string]*OutboundTrafficStats{}
	for _, stat := range response.GetStat() {
		tag, direction, ok := parseTrafficStatName(stat.GetName(), "outbound")
		if !ok {
			continue
		}
		item := byTag[tag]
		if item == nil {
			item = &OutboundTrafficStats{Outbound: tag}
			byTag[tag] = item
		}
		applyDirection(direction, stat.GetValue(), &item.Uplink, &item.Downlink)
	}

	outbounds := make([]OutboundTrafficStats, 0, len(byTag))
	for _, item := range byTag {
		outbounds = append(outbounds, *item)
	}
	sort.Slice(outbounds, func(i, j int) bool {
		return outbounds[i].Outbound < outbounds[j].Outbound
	})
	return outbounds, nil
}

func (c *statsClient) getValue(ctx context.Context, name string, reset bool) (int64, error) {
	response, err := c.raw.GetStats(ctx, &statscommand.GetStatsRequest{Name: name, Reset_: reset})
	if err != nil {
		return 0, fmt.Errorf("xray stats get %q: %w", name, err)
	}
	if response.GetStat() == nil {
		return 0, nil
	}
	return response.GetStat().GetValue(), nil
}

func (c *statsClient) query(ctx context.Context, pattern string, reset bool) (*statscommand.QueryStatsResponse, error) {
	response, err := c.raw.QueryStats(ctx, &statscommand.QueryStatsRequest{Pattern: pattern, Reset_: reset})
	if err != nil {
		return nil, fmt.Errorf("xray stats query %q: %w", pattern, err)
	}
	return response, nil
}

func (c *statsClient) onlineIPList(ctx context.Context, username string, reset bool) ([]IPLastSeen, error) {
	name := fmt.Sprintf(StatsUserOnlineFormat, username)
	response, err := c.raw.GetStatsOnlineIpList(ctx, &statscommand.GetStatsRequest{Name: name, Reset_: reset})
	if err != nil {
		return nil, fmt.Errorf("xray stats get online IP list for user %q: %w", username, err)
	}

	ips := make([]IPLastSeen, 0, len(response.GetIps()))
	for ip, lastSeen := range response.GetIps() {
		ips = append(ips, IPLastSeen{IP: ip, LastSeen: lastSeen})
	}
	sort.Slice(ips, func(i, j int) bool {
		if ips[i].LastSeen == ips[j].LastSeen {
			return ips[i].IP < ips[j].IP
		}
		return ips[i].LastSeen > ips[j].LastSeen
	})
	return ips, nil
}

func parseTrafficStatName(name string, prefix string) (tag string, direction string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[0] != prefix || parts[2] != "traffic" {
		return "", "", false
	}
	if parts[1] == "" || (parts[3] != "uplink" && parts[3] != "downlink") {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func parseOnlineStatName(name string) (username string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 3 || parts[0] != "user" || parts[1] == "" || parts[2] != "online" {
		return "", false
	}
	return parts[1], true
}

func applyDirection(direction string, value int64, uplink *int64, downlink *int64) {
	switch direction {
	case "uplink":
		*uplink += value
	case "downlink":
		*downlink += value
	}
}
