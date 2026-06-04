package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/version"
	"github.com/x-dora/rw-node-go/internal/xray"
)

func TestHealthcheckReturnsCachedStatusAndNodeVersion(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{}
	controller := XrayController{
		state:    runtimeState,
		logger:   slog.Default(),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/node/xray/healthcheck", nil)

	controller.Healthcheck(ctx)

	var body struct {
		Response struct {
			IsAlive                  bool   `json:"isAlive"`
			XrayInternalStatusCached bool   `json:"xrayInternalStatusCached"`
			NodeVersion              string `json:"nodeVersion"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !body.Response.IsAlive {
		t.Fatalf("IsAlive = false, want true")
	}
	if body.Response.XrayInternalStatusCached {
		t.Fatalf("XrayInternalStatusCached = true, want false")
	}
	if body.Response.NodeVersion != version.NodeVersion {
		t.Fatalf("NodeVersion = %q, want %q", body.Response.NodeVersion, version.NodeVersion)
	}
}

func TestStartXrayStartsCore(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{version: "25.1.1"}
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h1","inbounds":[]}},
		"xrayConfig":{}
	}`))

	controller.Start(ctx)

	var body struct {
		Response struct {
			IsStarted bool    `json:"isStarted"`
			Error     *string `json:"error"`
			System    struct {
				Stats struct {
					Interface *struct {
						Interface string `json:"interface"`
					} `json:"interface"`
				} `json:"stats"`
			} `json:"system"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !body.Response.IsStarted {
		t.Fatalf("IsStarted = false, want true; body=%s", rec.Body.String())
	}
	if body.Response.Error != nil {
		t.Fatalf("Error = %v, want nil", *body.Response.Error)
	}
	if body.Response.System.Stats.Interface == nil || body.Response.System.Stats.Interface.Interface != "eth0" {
		t.Fatalf("system = %#v; body=%s", body.Response.System, rec.Body.String())
	}
	if !core.started {
		t.Fatalf("core was not started")
	}
	snapshot := runtimeState.Snapshot()
	if !snapshot.XrayInternalStatusCached {
		t.Fatalf("XrayInternalStatusCached = false, want true")
	}
	logs := logBuffer.String()
	for _, want := range []string{"Xray start request", "Xray config received", "Stats User Online Enabled", "Xray started", "25.1.1"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
}

func TestStatsUserOnlineEnabledReadsFinalPolicy(t *testing.T) {
	enabledConfig := map[string]any{
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{"statsUserOnline": true},
			},
		},
	}
	if !statsUserOnlineEnabled(enabledConfig) {
		t.Fatalf("statsUserOnlineEnabled() = false, want true")
	}

	for name, config := range map[string]map[string]any{
		"disabled": {
			"policy": map[string]any{
				"levels": map[string]any{
					"0": map[string]any{"statsUserOnline": false},
				},
			},
		},
		"missing policy": {},
		"missing level": {
			"policy": map[string]any{"levels": map[string]any{}},
		},
		"non bool": {
			"policy": map[string]any{
				"levels": map[string]any{
					"0": map[string]any{"statsUserOnline": "true"},
				},
			},
		},
	} {
		if statsUserOnlineEnabled(config) {
			t.Fatalf("%s: statsUserOnlineEnabled() = true, want false", name)
		}
	}
}

func TestStartXrayLogsInboundSummary(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{version: "25.1.1"}
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h1","inbounds":[{"tag":"VLESS_INBOUND","usersCount":2,"hash":"1234567890abcdef"}]}},
		"xrayConfig":{"inbounds":[{"tag":"VLESS_INBOUND","protocol":"vless","settings":{"clients":[{"id":"client-secret"}]}}]}
	}`))

	controller.Start(ctx)

	logs := logBuffer.String()
	for _, want := range []string{"VLESS_INBOUND", "users=2", "1234567890ab"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "client-secret") {
		t.Fatalf("logs leaked xray client id:\n%s", logs)
	}
}

func TestStartXrayReturnsErrorWhenCoreFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     &fakeCore{startErr: errors.New("boom")},
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h1","inbounds":[]}},
		"xrayConfig":{}
	}`))

	controller.Start(ctx)

	var body struct {
		Response struct {
			IsStarted bool    `json:"isStarted"`
			Error     *string `json:"error"`
			System    struct {
				Stats struct {
					Interface *struct {
						Interface string `json:"interface"`
					} `json:"interface"`
				} `json:"stats"`
			} `json:"system"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Response.IsStarted {
		t.Fatalf("IsStarted = true, want false")
	}
	if body.Response.Error == nil || *body.Response.Error != "boom" {
		t.Fatalf("Error = %v, want boom", body.Response.Error)
	}
	if body.Response.System.Stats.Interface == nil || body.Response.System.Stats.Interface.Interface != "eth0" {
		t.Fatalf("system = %#v; body=%s", body.Response.System, rec.Body.String())
	}
	if runtimeState.Snapshot().XrayInternalStatusCached {
		t.Fatalf("XrayInternalStatusCached = true, want false after start failure")
	}
	logs := logBuffer.String()
	for _, want := range []string{"Xray failed to start", "boom"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
}

func TestStartXrayRedactsSensitiveCoreErrorFromLogs(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	sensitiveErr := errors.New(`parse xray config: clients[0].id="client-secret" password=trojan-password privateKey=super-private-key`)
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     &fakeCore{startErr: sensitiveErr},
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h1","inbounds":[]}},
		"xrayConfig":{}
	}`))

	controller.Start(ctx)

	logs := logBuffer.String()
	for _, leaked := range []string{"client-secret", "trojan-password", "super-private-key"} {
		if strings.Contains(logs, leaked) {
			t.Fatalf("logs leaked %q:\n%s", leaked, logs)
		}
	}
	for _, want := range []string{"Xray failed to start", "[REDACTED]"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}

	var body struct {
		Response struct {
			Error *string `json:"error"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Response.Error == nil || !strings.Contains(*body.Response.Error, "client-secret") {
		t.Fatalf("response error = %v, want existing raw contract error", body.Response.Error)
	}
}

func TestStartXrayFailureKeepsLastConfigForDiagnostics(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	versionValue := "25.1.1"
	lastConfig := map[string]any{"inbounds": []any{map[string]any{"tag": "VLESS_INBOUND"}}}
	lastHashes := state.Hashes{EmptyConfig: "old"}
	runtimeState.SetXrayStarted(&versionValue, lastConfig, lastHashes)
	controller := XrayController{
		state:    runtimeState,
		logger:   slog.Default(),
		core:     &fakeCore{started: true, startErr: errors.New("boom")},
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":true,"hashes":{"emptyConfig":"new","inbounds":[]}},
		"xrayConfig":{}
	}`))

	controller.Start(ctx)

	snapshot := runtimeState.Snapshot()
	if snapshot.XrayRunning || snapshot.XrayInternalStatusCached {
		t.Fatalf("running=%v cached=%v, want both false", snapshot.XrayRunning, snapshot.XrayInternalStatusCached)
	}
	if snapshot.XrayVersion == nil || *snapshot.XrayVersion != versionValue {
		t.Fatalf("XrayVersion = %v, want previous version", snapshot.XrayVersion)
	}
	if snapshot.LastHashes.EmptyConfig != lastHashes.EmptyConfig {
		t.Fatalf("LastHashes = %#v, want previous hashes %#v", snapshot.LastHashes, lastHashes)
	}
	inbounds, ok := snapshot.CurrentConfig["inbounds"].([]any)
	if !ok || len(inbounds) != 1 {
		t.Fatalf("CurrentConfig = %#v, want previous config", snapshot.CurrentConfig)
	}
}

func TestStopXrayClearsHealthcheckInternalStatus(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	versionValue := "25.1.1"
	runtimeState.SetXrayStarted(&versionValue, map[string]any{}, state.Hashes{EmptyConfig: "h1"})
	core := &fakeCore{started: true}
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}

	stopRec := httptest.NewRecorder()
	stopCtx, _ := gin.CreateTestContext(stopRec)
	stopCtx.Request = httptest.NewRequest(http.MethodGet, "/node/xray/stop", nil)
	controller.Stop(stopCtx)

	healthRec := httptest.NewRecorder()
	healthCtx, _ := gin.CreateTestContext(healthRec)
	healthCtx.Request = httptest.NewRequest(http.MethodGet, "/node/xray/healthcheck", nil)
	controller.Healthcheck(healthCtx)

	var body struct {
		Response struct {
			XrayInternalStatusCached bool `json:"xrayInternalStatusCached"`
		} `json:"response"`
	}
	if err := json.Unmarshal(healthRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal healthcheck: %v; body=%s", err, healthRec.Body.String())
	}
	if body.Response.XrayInternalStatusCached {
		t.Fatalf("XrayInternalStatusCached = true, want false")
	}
	logs := logBuffer.String()
	for _, want := range []string{"Remnawave requested to stop Xray", "Xray stopped"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
}

func TestStartXraySkipsRestartWhenHashesUnchanged(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	hashes := state.Hashes{EmptyConfig: "h1"}
	versionValue := "25.1.1"
	runtimeState.SetXrayStarted(&versionValue, map[string]any{}, hashes)
	core := &fakeCore{started: true, version: "25.1.1"}
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h1","inbounds":[]}},
		"xrayConfig":{"inbounds":[{"tag":"REMNAWAVE_API_INBOUND"}]}
	}`))

	controller.Start(ctx)

	var body struct {
		Response struct {
			IsStarted bool    `json:"isStarted"`
			Error     *string `json:"error"`
			System    struct {
				Stats struct {
					Interface *struct {
						Interface string `json:"interface"`
					} `json:"interface"`
				} `json:"stats"`
			} `json:"system"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Response.IsStarted || body.Response.Error != nil {
		t.Fatalf("response = %s", rec.Body.String())
	}
	if body.Response.System.Stats.Interface == nil || body.Response.System.Stats.Interface.Interface != "eth0" {
		t.Fatalf("system = %#v; body=%s", body.Response.System, rec.Body.String())
	}
	if core.startCalls != 0 {
		t.Fatalf("startCalls = %d, want 0", core.startCalls)
	}
	if logs := logBuffer.String(); !strings.Contains(logs, "configuration is up-to-date - no restart required") {
		t.Fatalf("logs missing no-restart message:\n%s", logs)
	}
}

func TestStartXrayRestartsWhenHashesUnchangedButHealthFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	hashes := state.Hashes{EmptyConfig: "h1"}
	versionValue := "25.1.1"
	runtimeState.SetXrayStarted(&versionValue, map[string]any{}, hashes)
	core := &fakeCore{started: true, version: "25.1.1", healthErr: errors.New("not healthy")}
	controller := XrayController{
		state:    runtimeState,
		logger:   slog.Default(),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h1","inbounds":[]}},
		"xrayConfig":{}
	}`))

	controller.Start(ctx)

	if core.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", core.startCalls)
	}
	var body struct {
		Response struct {
			IsStarted bool    `json:"isStarted"`
			Error     *string `json:"error"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Response.IsStarted || body.Response.Error != nil {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestStartXrayLogsForceRestart(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	versionValue := "25.1.1"
	runtimeState.SetXrayStarted(&versionValue, map[string]any{}, state.Hashes{EmptyConfig: "old"})
	core := &fakeCore{started: true, version: "25.1.1"}
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":true,"hashes":{"emptyConfig":"new","inbounds":[]}},
		"xrayConfig":{}
	}`))

	controller.Start(ctx)

	if logs := logBuffer.String(); !strings.Contains(logs, "Force restart requested") {
		t.Fatalf("logs missing force restart message:\n%s", logs)
	}
}

func TestHandlerStubEnvelope(t *testing.T) {
	t.Skip("handler stubs are covered by route envelope tests")
}

type fakeCore struct {
	started    bool
	version    string
	startErr   error
	healthErr  error
	startCalls int
	config     []byte
	handler    xray.HandlerClient
	stats      xray.StatsClient
	routing    xray.RoutingClient
}

func (f *fakeCore) Start(ctx context.Context, config []byte) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.startCalls++
	f.started = true
	f.healthErr = nil
	f.config = append([]byte(nil), config...)
	return nil
}

func (f *fakeCore) Stop(ctx context.Context) error {
	f.started = false
	return nil
}

func (f *fakeCore) IsRunning() bool {
	return f.started
}

func (f *fakeCore) Health(ctx context.Context) error {
	return f.healthErr
}

func (f *fakeCore) Version(ctx context.Context) (string, error) {
	return f.version, nil
}

func (f *fakeCore) Handler() xray.HandlerClient {
	return f.handler
}

func (f *fakeCore) Stats() xray.StatsClient {
	return f.stats
}

func (f *fakeCore) Routing() xray.RoutingClient {
	return f.routing
}

func testLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
