package xray

import (
	"context"
	"errors"
	"testing"
	"time"

	appstats "github.com/xtls/xray-core/app/stats"
	xcore "github.com/xtls/xray-core/core"
)

func TestParseTrafficStatName(t *testing.T) {
	tag, direction, ok := parseTrafficStatName("user>>>alpha>>>traffic>>>uplink", "user")
	if !ok || tag != "alpha" || direction != "uplink" {
		t.Fatalf("parseTrafficStatName() = %q %q %v", tag, direction, ok)
	}
	if _, _, ok := parseTrafficStatName("user>>>alpha>>>online", "user"); ok {
		t.Fatalf("parseTrafficStatName() accepted online stat")
	}
}

func TestParseOnlineStatName(t *testing.T) {
	username, ok := parseOnlineStatName("user>>>alpha>>>online")
	if !ok || username != "alpha" {
		t.Fatalf("parseOnlineStatName() = %q %v", username, ok)
	}

	for _, input := range []string{
		"user>>>>>>online",
		"user>>>alpha>>>traffic>>>uplink",
		"inbound>>>alpha>>>online",
		"user>>>alpha>>>offline",
	} {
		if username, ok := parseOnlineStatName(input); ok {
			t.Fatalf("parseOnlineStatName(%q) = %q, true; want false", input, username)
		}
	}
}

func TestApplyDirection(t *testing.T) {
	var uplink, downlink int64
	applyDirection("uplink", 10, &uplink, &downlink)
	applyDirection("downlink", 20, &uplink, &downlink)
	if uplink != 10 || downlink != 20 {
		t.Fatalf("uplink=%d downlink=%d", uplink, downlink)
	}
}

func TestEmbeddedStatsOnlineMap(t *testing.T) {
	manager := newStatsManager(t)
	alphaMap := registerOnlineMap(t, manager, "user>>>alpha>>>online")
	alphaMap.AddIP("203.0.113.20")
	alphaMap.AddIP("203.0.113.10")
	emptyMap := registerOnlineMap(t, manager, "user>>>empty>>>online")
	emptyMap.AddIP("203.0.113.30")
	emptyMap.RemoveIP("203.0.113.30")
	malformedMap := registerOnlineMap(t, manager, "system>>>bad>>>online")
	malformedMap.AddIP("203.0.113.40")

	if !onlineStatus(manager, "alpha") {
		t.Fatalf("onlineStatus(alpha) = false, want true")
	}
	if onlineStatus(manager, "missing") {
		t.Fatalf("onlineStatus(missing) = true, want false")
	}

	ips := onlineIPList(manager, "alpha")
	if len(ips) != 2 || ips[0].IP != "203.0.113.10" || ips[1].IP != "203.0.113.20" {
		t.Fatalf("onlineIPList(alpha) = %#v", ips)
	}
	if ips[0].LastSeen == 0 || ips[1].LastSeen == 0 {
		t.Fatalf("onlineIPList(alpha) missing lastSeen: %#v", ips)
	}

	missingIPs := onlineIPList(manager, "missing")
	if len(missingIPs) != 0 {
		t.Fatalf("onlineIPList(missing) = %#v, want empty", missingIPs)
	}

	users := onlineUsersIPList(manager)
	if len(users) != 1 || users[0].Username != "alpha" {
		t.Fatalf("onlineUsersIPList() = %#v", users)
	}
	if len(users[0].IPs) != 2 || users[0].IPs[0].IP != "203.0.113.10" || users[0].IPs[1].IP != "203.0.113.20" {
		t.Fatalf("onlineUsersIPList() IPs = %#v", users[0].IPs)
	}
}

func TestEmbeddedCoreUptime(t *testing.T) {
	core := &EmbeddedCore{
		instance:  &xcore.Instance{},
		startedAt: time.Now().Add(-3 * time.Second),
	}
	stats, err := (&embeddedStatsClient{core: core}).SysStats(context.Background())
	if err != nil {
		t.Fatalf("SysStats() error = %v", err)
	}
	if stats.Uptime < 3 {
		t.Fatalf("SysStats().Uptime = %d, want at least 3", stats.Uptime)
	}
}

func TestEmbeddedCoreStartKeepsOldInstanceWhenConfigLoadFails(t *testing.T) {
	oldInstance := &xcore.Instance{}
	core := &EmbeddedCore{instance: oldInstance, startedAt: time.Now().Add(-3 * time.Second)}
	loadErr := errors.New("load boom")
	var closed []*xcore.Instance

	restore := overrideEmbeddedCoreHooks(t, embeddedCoreHooks{
		loadErr: loadErr,
		closeFn: func(instance *xcore.Instance) error {
			closed = append(closed, instance)
			return nil
		},
	})
	defer restore()

	err := core.Start(context.Background(), []byte(`{}`))
	if err == nil || !errors.Is(err, loadErr) {
		t.Fatalf("Start() error = %v, want wrapped load error", err)
	}
	if !core.IsRunning() || core.Instance() != oldInstance {
		t.Fatalf("old instance was not preserved after load failure")
	}
	if len(closed) != 0 {
		t.Fatalf("closed instances = %#v, want none", closed)
	}
}

func TestEmbeddedCoreStartStopsOldInstanceBeforeStartingNewInstance(t *testing.T) {
	oldInstance := &xcore.Instance{}
	newInstance := &xcore.Instance{}
	core := &EmbeddedCore{instance: oldInstance, startedAt: time.Now().Add(-3 * time.Second)}
	var closed []*xcore.Instance
	oldClosedBeforeStart := false

	restore := overrideEmbeddedCoreHooks(t, embeddedCoreHooks{
		newInstance: newInstance,
		version:     "test-version",
		startFn: func(instance *xcore.Instance) error {
			oldClosedBeforeStart = len(closed) == 1 && closed[0] == oldInstance
			return nil
		},
		closeFn: func(instance *xcore.Instance) error {
			closed = append(closed, instance)
			return nil
		},
	})
	defer restore()

	if err := core.Start(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !core.IsRunning() || core.Instance() != newInstance {
		t.Fatalf("new instance was not installed")
	}
	version, err := core.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version != "test-version" {
		t.Fatalf("Version() = %q, want test-version", version)
	}
	if !oldClosedBeforeStart {
		t.Fatalf("old instance was not closed before new start")
	}
	if len(closed) != 1 || closed[0] != oldInstance {
		t.Fatalf("closed instances = %#v, want only old instance", closed)
	}
}

func TestEmbeddedCoreStartFailureAfterOldInstanceStoppedStaysStopped(t *testing.T) {
	oldInstance := &xcore.Instance{}
	newInstance := &xcore.Instance{}
	core := &EmbeddedCore{instance: oldInstance, startedAt: time.Now().Add(-3 * time.Second)}
	startErr := errors.New("start boom")
	var closed []*xcore.Instance

	restore := overrideEmbeddedCoreHooks(t, embeddedCoreHooks{
		newInstance: newInstance,
		startErr:    startErr,
		closeFn: func(instance *xcore.Instance) error {
			closed = append(closed, instance)
			return nil
		},
	})
	defer restore()

	err := core.Start(context.Background(), []byte(`{}`))
	if err == nil || !errors.Is(err, startErr) {
		t.Fatalf("Start() error = %v, want wrapped start error", err)
	}
	if core.IsRunning() || core.Instance() != nil {
		t.Fatalf("core is running after failed start")
	}
	if len(closed) != 2 || closed[0] != oldInstance || closed[1] != newInstance {
		t.Fatalf("closed instances = %#v, want old then new", closed)
	}
}

func TestEmbeddedCoreUptimeStopsWhenCoreIsStopped(t *testing.T) {
	core := &EmbeddedCore{
		instance:  &xcore.Instance{},
		startedAt: time.Now().Add(-3 * time.Second),
	}
	if uptime := core.uptime(time.Now()); uptime == 0 {
		t.Fatalf("uptime before stop = 0, want non-zero")
	}

	if err := core.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if uptime := core.uptime(time.Now().Add(3 * time.Second)); uptime != 0 {
		t.Fatalf("uptime after stop = %d, want 0", uptime)
	}
	if !core.startedAt.IsZero() {
		t.Fatalf("startedAt after stop = %v, want zero", core.startedAt)
	}
}

func TestEmbeddedCoreUptimeUsesLatestStartTime(t *testing.T) {
	core := &EmbeddedCore{
		instance:  &xcore.Instance{},
		startedAt: time.Now().Add(-30 * time.Second),
	}
	core.mu.Lock()
	core.instance = &xcore.Instance{}
	core.startedAt = time.Now().Add(-2 * time.Second)
	core.mu.Unlock()

	uptime := core.uptime(time.Now())
	if uptime < 2 || uptime >= 30 {
		t.Fatalf("uptime = %d, want latest start time", uptime)
	}
}

func TestEmbeddedCoreUptimeFallsBackToZero(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		core *EmbeddedCore
	}{
		{
			name: "not running",
			core: &EmbeddedCore{startedAt: now.Add(-3 * time.Second)},
		},
		{
			name: "missing start time",
			core: &EmbeddedCore{instance: &xcore.Instance{}},
		},
		{
			name: "future start time",
			core: &EmbeddedCore{instance: &xcore.Instance{}, startedAt: now.Add(time.Second)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if uptime := tt.core.uptime(now); uptime != 0 {
				t.Fatalf("uptime = %d, want 0", uptime)
			}
		})
	}
}

func newStatsManager(t *testing.T) *appstats.Manager {
	t.Helper()
	manager, err := appstats.NewManager(context.Background(), &appstats.Config{})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func registerOnlineMap(t *testing.T, manager *appstats.Manager, name string) appstatsOnlineMap {
	t.Helper()
	onlineMap, err := manager.RegisterOnlineMap(name)
	if err != nil {
		t.Fatalf("RegisterOnlineMap(%q) error = %v", name, err)
	}
	return onlineMap
}

type appstatsOnlineMap interface {
	AddIP(string)
	RemoveIP(string)
}

type embeddedCoreHooks struct {
	newInstance *xcore.Instance
	loadErr     error
	newErr      error
	startErr    error
	startFn     func(*xcore.Instance) error
	version     string
	closeFn     func(*xcore.Instance) error
}

func overrideEmbeddedCoreHooks(t *testing.T, hooks embeddedCoreHooks) func() {
	t.Helper()
	previousLoad := loadXrayConfig
	previousNew := newXrayInstance
	previousStart := startXrayInstance
	previousClose := closeXrayInstance
	previousVersion := xrayCoreVersion

	loadXrayConfig = func(formatName string, input any) (*xcore.Config, error) {
		if hooks.loadErr != nil {
			return nil, hooks.loadErr
		}
		return &xcore.Config{}, nil
	}
	newXrayInstance = func(config *xcore.Config) (*xcore.Instance, error) {
		if hooks.newErr != nil {
			return nil, hooks.newErr
		}
		return hooks.newInstance, nil
	}
	startXrayInstance = func(instance *xcore.Instance) error {
		if hooks.startFn != nil {
			return hooks.startFn(instance)
		}
		return hooks.startErr
	}
	closeXrayInstance = func(instance *xcore.Instance) error {
		if hooks.closeFn != nil {
			return hooks.closeFn(instance)
		}
		return nil
	}
	xrayCoreVersion = func() string {
		if hooks.version != "" {
			return hooks.version
		}
		return "test-version"
	}

	return func() {
		loadXrayConfig = previousLoad
		newXrayInstance = previousNew
		startXrayInstance = previousStart
		closeXrayInstance = previousClose
		xrayCoreVersion = previousVersion
	}
}
