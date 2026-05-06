package xray

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"sync"

	appstats "github.com/xtls/xray-core/app/stats"
	xcore "github.com/xtls/xray-core/core"
	featureinbound "github.com/xtls/xray-core/features/inbound"
	featurestats "github.com/xtls/xray-core/features/stats"
	_ "github.com/xtls/xray-core/main/distro/all"
	"github.com/xtls/xray-core/proxy"
)

type Core interface {
	Start(ctx context.Context, config []byte) error
	Stop(ctx context.Context) error
	IsRunning() bool
	Health(ctx context.Context) error
	Version(ctx context.Context) (string, error)
	Handler() HandlerClient
	Stats() StatsClient
	Routing() RoutingClient
}

type EmbeddedCore struct {
	mu       sync.RWMutex
	instance *xcore.Instance
	version  string
}

func NewEmbeddedCore() *EmbeddedCore {
	return &EmbeddedCore{version: xcore.Version()}
}

func (c *EmbeddedCore) Start(ctx context.Context, configJSON []byte) error {
	if !json.Valid(configJSON) {
		return fmt.Errorf("xray config is not valid JSON")
	}

	config, err := xcore.LoadConfig("json", bytes.NewReader(configJSON))
	if err != nil {
		return fmt.Errorf("load embedded xray config: %w", err)
	}
	instance, err := xcore.New(config)
	if err != nil {
		return fmt.Errorf("create embedded xray instance: %w", err)
	}

	c.mu.Lock()
	old := c.instance
	c.instance = nil
	c.mu.Unlock()
	if old != nil {
		old.Close()
	}

	if err := instance.Start(); err != nil {
		instance.Close()
		return fmt.Errorf("start embedded xray instance: %w", err)
	}

	c.mu.Lock()
	c.instance = instance
	c.version = xcore.Version()
	c.mu.Unlock()
	return nil
}

func (c *EmbeddedCore) Stop(ctx context.Context) error {
	c.mu.Lock()
	instance := c.instance
	c.instance = nil
	c.mu.Unlock()
	if instance == nil {
		return nil
	}
	if err := instance.Close(); err != nil {
		return fmt.Errorf("stop embedded xray instance: %w", err)
	}
	return nil
}

func (c *EmbeddedCore) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instance != nil
}

func (c *EmbeddedCore) Instance() *xcore.Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instance
}

func (c *EmbeddedCore) Health(ctx context.Context) error {
	if !c.IsRunning() {
		return fmt.Errorf("xray is not running")
	}
	return nil
}

func (c *EmbeddedCore) Version(ctx context.Context) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.version != "" {
		return c.version, nil
	}
	return xcore.Version(), nil
}

func (c *EmbeddedCore) Handler() HandlerClient {
	return &embeddedHandlerClient{core: c}
}

func (c *EmbeddedCore) Stats() StatsClient {
	return &embeddedStatsClient{core: c}
}

func (c *EmbeddedCore) Routing() RoutingClient {
	return embeddedRoutingClient{core: c}
}

type HandlerClient interface {
	AddUser(ctx context.Context, spec UserSpec) error
	RemoveUser(ctx context.Context, tag string, username string) error
	GetInboundUsers(ctx context.Context, tag string) ([]InboundUser, error)
	GetInboundUsersCount(ctx context.Context, tag string) (int, error)
}

type StatsClient interface {
	Ping(ctx context.Context) error
	SysStats(ctx context.Context) (SysStats, error)
	UsersStats(ctx context.Context, reset bool) ([]UserTrafficStats, error)
	UserOnlineStatus(ctx context.Context, username string) (bool, error)
	UserIPList(ctx context.Context, username string, reset bool) ([]IPLastSeen, error)
	UsersIPList(ctx context.Context, reset bool) ([]UserIPList, error)
	InboundStats(ctx context.Context, tag string, reset bool) (InboundTrafficStats, error)
	OutboundStats(ctx context.Context, tag string, reset bool) (OutboundTrafficStats, error)
	AllInboundStats(ctx context.Context, reset bool) ([]InboundTrafficStats, error)
	AllOutboundStats(ctx context.Context, reset bool) ([]OutboundTrafficStats, error)
}

type RoutingClient interface {
	AddSourceIPRule(ctx context.Context, ruleTag string, sourceIP string, outboundTag string) error
	RemoveRule(ctx context.Context, ruleTag string) error
}

type embeddedStatsClient struct {
	core *EmbeddedCore
}

type embeddedHandlerClient struct {
	core *EmbeddedCore
}

func (c *embeddedHandlerClient) manager(tag string) (proxy.UserManager, error) {
	instance := c.core.Instance()
	if instance == nil {
		return nil, fmt.Errorf("xray is not running")
	}
	feature := instance.GetFeature(featureinbound.ManagerType())
	if feature == nil {
		return nil, fmt.Errorf("xray inbound manager is unavailable")
	}
	inboundManager, ok := feature.(featureinbound.Manager)
	if !ok {
		return nil, fmt.Errorf("xray inbound manager has unexpected type")
	}
	handler, err := inboundManager.GetHandler(context.Background(), tag)
	if err != nil {
		return nil, fmt.Errorf("xray inbound %q is unavailable: %w", tag, err)
	}
	inbound, ok := handler.(proxy.GetInbound)
	if !ok {
		return nil, fmt.Errorf("xray inbound %q does not expose users", tag)
	}
	userManager, ok := inbound.GetInbound().(proxy.UserManager)
	if !ok {
		return nil, fmt.Errorf("xray inbound %q does not support dynamic users", tag)
	}
	return userManager, nil
}

func (c *embeddedHandlerClient) AddUser(ctx context.Context, spec UserSpec) error {
	manager, err := c.manager(spec.Tag)
	if err != nil {
		return err
	}
	user, err := BuildProtocolUser(spec)
	if err != nil {
		return err
	}
	memoryUser, err := user.ToMemoryUser()
	if err != nil {
		return fmt.Errorf("convert xray user %q: %w", spec.Username, err)
	}
	if err := manager.AddUser(ctx, memoryUser); err != nil {
		return fmt.Errorf("xray handler add user to inbound %q: %w", spec.Tag, err)
	}
	return nil
}

func (c *embeddedHandlerClient) RemoveUser(ctx context.Context, tag string, username string) error {
	manager, err := c.manager(tag)
	if err != nil {
		return err
	}
	if err := manager.RemoveUser(ctx, username); err != nil {
		return fmt.Errorf("xray handler remove user from inbound %q: %w", tag, err)
	}
	return nil
}

func (c *embeddedHandlerClient) GetInboundUsers(ctx context.Context, tag string) ([]InboundUser, error) {
	manager, err := c.manager(tag)
	if err != nil {
		return nil, err
	}
	users := manager.GetUsers(ctx)
	output := make([]InboundUser, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		output = append(output, InboundUser{
			Username: user.Email,
			Email:    user.Email,
			Level:    int(user.Level),
		})
	}
	return output, nil
}

func (c *embeddedHandlerClient) GetInboundUsersCount(ctx context.Context, tag string) (int, error) {
	manager, err := c.manager(tag)
	if err != nil {
		return 0, err
	}
	return int(manager.GetUsersCount(ctx)), nil
}

func (c *embeddedStatsClient) Ping(ctx context.Context) error {
	if !c.core.IsRunning() {
		return fmt.Errorf("xray is not running")
	}
	return nil
}

func (c *embeddedStatsClient) SysStats(ctx context.Context) (SysStats, error) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return SysStats{
		NumGoroutine: uint32(runtime.NumGoroutine()),
		NumGC:        stats.NumGC,
		Alloc:        stats.Alloc,
		TotalAlloc:   stats.TotalAlloc,
		Sys:          stats.Sys,
		Mallocs:      stats.Mallocs,
		Frees:        stats.Frees,
		LiveObjects:  stats.Mallocs - stats.Frees,
		PauseTotalNs: stats.PauseTotalNs,
	}, nil
}

func (c *embeddedStatsClient) manager() (*appstats.Manager, error) {
	instance := c.core.Instance()
	if instance == nil {
		return nil, fmt.Errorf("xray is not running")
	}
	feature := instance.GetFeature(featurestats.ManagerType())
	if feature == nil {
		return nil, fmt.Errorf("xray stats manager is unavailable")
	}
	manager, ok := feature.(*appstats.Manager)
	if !ok {
		return nil, fmt.Errorf("xray stats manager has unexpected type")
	}
	return manager, nil
}

func (c *embeddedStatsClient) UsersStats(ctx context.Context, reset bool) ([]UserTrafficStats, error) {
	manager, err := c.manager()
	if err != nil {
		return nil, err
	}
	byUser := map[string]*UserTrafficStats{}
	manager.VisitCounters(func(name string, counter featurestats.Counter) bool {
		username, direction, ok := parseTrafficStatName(name, "user")
		if !ok {
			return true
		}
		item := byUser[username]
		if item == nil {
			item = &UserTrafficStats{Username: username}
			byUser[username] = item
		}
		value := counter.Value()
		if reset {
			counter.Set(0)
		}
		applyDirection(direction, value, &item.Uplink, &item.Downlink)
		return true
	})
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

func (c *embeddedStatsClient) UserOnlineStatus(ctx context.Context, username string) (bool, error) {
	manager, err := c.manager()
	if err != nil {
		return false, err
	}
	return onlineStatus(manager, username), nil
}

func (c *embeddedStatsClient) UserIPList(ctx context.Context, username string, reset bool) ([]IPLastSeen, error) {
	manager, err := c.manager()
	if err != nil {
		return nil, err
	}
	return onlineIPList(manager, username), nil
}

func (c *embeddedStatsClient) UsersIPList(ctx context.Context, reset bool) ([]UserIPList, error) {
	manager, err := c.manager()
	if err != nil {
		return nil, err
	}
	return onlineUsersIPList(manager), nil
}

func (c *embeddedStatsClient) InboundStats(ctx context.Context, tag string, reset bool) (InboundTrafficStats, error) {
	manager, err := c.manager()
	if err != nil {
		return InboundTrafficStats{}, err
	}
	uplink := embeddedCounterValue(manager, fmt.Sprintf(StatsInboundUplinkFormat, tag), reset)
	downlink := embeddedCounterValue(manager, fmt.Sprintf(StatsInboundDownlinkFormat, tag), reset)
	return InboundTrafficStats{Inbound: tag, Uplink: uplink, Downlink: downlink}, nil
}

func (c *embeddedStatsClient) OutboundStats(ctx context.Context, tag string, reset bool) (OutboundTrafficStats, error) {
	manager, err := c.manager()
	if err != nil {
		return OutboundTrafficStats{}, err
	}
	uplink := embeddedCounterValue(manager, fmt.Sprintf(StatsOutboundUplinkFormat, tag), reset)
	downlink := embeddedCounterValue(manager, fmt.Sprintf(StatsOutboundDownlinkFormat, tag), reset)
	return OutboundTrafficStats{Outbound: tag, Uplink: uplink, Downlink: downlink}, nil
}

func (c *embeddedStatsClient) AllInboundStats(ctx context.Context, reset bool) ([]InboundTrafficStats, error) {
	manager, err := c.manager()
	if err != nil {
		return nil, err
	}
	byTag := map[string]*InboundTrafficStats{}
	manager.VisitCounters(func(name string, counter featurestats.Counter) bool {
		tag, direction, ok := parseTrafficStatName(name, "inbound")
		if !ok {
			return true
		}
		item := byTag[tag]
		if item == nil {
			item = &InboundTrafficStats{Inbound: tag}
			byTag[tag] = item
		}
		value := counter.Value()
		if reset {
			counter.Set(0)
		}
		applyDirection(direction, value, &item.Uplink, &item.Downlink)
		return true
	})
	inbounds := make([]InboundTrafficStats, 0, len(byTag))
	for _, item := range byTag {
		inbounds = append(inbounds, *item)
	}
	sort.Slice(inbounds, func(i, j int) bool {
		return inbounds[i].Inbound < inbounds[j].Inbound
	})
	return inbounds, nil
}

func (c *embeddedStatsClient) AllOutboundStats(ctx context.Context, reset bool) ([]OutboundTrafficStats, error) {
	manager, err := c.manager()
	if err != nil {
		return nil, err
	}
	byTag := map[string]*OutboundTrafficStats{}
	manager.VisitCounters(func(name string, counter featurestats.Counter) bool {
		tag, direction, ok := parseTrafficStatName(name, "outbound")
		if !ok {
			return true
		}
		item := byTag[tag]
		if item == nil {
			item = &OutboundTrafficStats{Outbound: tag}
			byTag[tag] = item
		}
		value := counter.Value()
		if reset {
			counter.Set(0)
		}
		applyDirection(direction, value, &item.Uplink, &item.Downlink)
		return true
	})
	outbounds := make([]OutboundTrafficStats, 0, len(byTag))
	for _, item := range byTag {
		outbounds = append(outbounds, *item)
	}
	sort.Slice(outbounds, func(i, j int) bool {
		return outbounds[i].Outbound < outbounds[j].Outbound
	})
	return outbounds, nil
}

func embeddedCounterValue(manager *appstats.Manager, name string, reset bool) int64 {
	counter := manager.GetCounter(name)
	if counter == nil {
		return 0
	}
	value := counter.Value()
	if reset {
		counter.Set(0)
	}
	return value
}

func onlineStatus(manager featurestats.Manager, username string) bool {
	onlineMap := manager.GetOnlineMap(fmt.Sprintf(StatsUserOnlineFormat, username))
	return onlineMap != nil && onlineMap.Count() > 0
}

func onlineIPList(manager featurestats.Manager, username string) []IPLastSeen {
	return onlineIPLastSeen(manager.GetOnlineMap(fmt.Sprintf(StatsUserOnlineFormat, username)))
}

func onlineUsersIPList(manager featurestats.Manager) []UserIPList {
	rawUsers := manager.GetAllOnlineUsers()
	output := make([]UserIPList, 0, len(rawUsers))
	seen := map[string]struct{}{}
	for _, raw := range rawUsers {
		username, ok := parseOnlineStatName(raw)
		if !ok {
			continue
		}
		if _, ok := seen[username]; ok {
			continue
		}
		seen[username] = struct{}{}
		ips := onlineIPLastSeen(manager.GetOnlineMap(raw))
		if len(ips) == 0 {
			continue
		}
		output = append(output, UserIPList{Username: username, IPs: ips})
	}
	sort.Slice(output, func(i, j int) bool {
		return output[i].Username < output[j].Username
	})
	return output
}

func onlineIPLastSeen(onlineMap featurestats.OnlineMap) []IPLastSeen {
	if onlineMap == nil {
		return []IPLastSeen{}
	}
	ipTimes := onlineMap.IPTimeMap()
	output := make([]IPLastSeen, 0, len(ipTimes))
	for ip, lastSeen := range ipTimes {
		output = append(output, IPLastSeen{IP: ip, LastSeen: lastSeen.Unix()})
	}
	sort.Slice(output, func(i, j int) bool {
		return output[i].IP < output[j].IP
	})
	return output
}

type embeddedRoutingClient struct {
	core *EmbeddedCore
}
