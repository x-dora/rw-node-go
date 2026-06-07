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
	"sync"
	"testing"
	"time"

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

func TestStartXrayFailureKeepsCachedStatusWhenOldCoreStillRunning(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	var logBuffer bytes.Buffer
	runtimeState := state.NewRuntimeState()
	versionValue := "25.1.1"
	runtimeState.SetXrayStarted(&versionValue, map[string]any{}, state.Hashes{EmptyConfig: "old"})
	controller := XrayController{
		state:    runtimeState,
		logger:   testLogger(&logBuffer),
		core:     &fakeCore{started: true, version: "25.1.1", startErr: errors.New("load boom"), startErrLeavesRunning: true},
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

	var body struct {
		Response struct {
			IsStarted bool    `json:"isStarted"`
			Error     *string `json:"error"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if body.Response.IsStarted || body.Response.Error == nil || *body.Response.Error != "load boom" {
		t.Fatalf("response = %s", rec.Body.String())
	}
	snapshot := runtimeState.Snapshot()
	if !snapshot.XrayRunning || !snapshot.XrayInternalStatusCached {
		t.Fatalf("running=%v cached=%v, want both true while old core still runs", snapshot.XrayRunning, snapshot.XrayInternalStatusCached)
	}
	if snapshot.XrayVersion == nil || *snapshot.XrayVersion != versionValue {
		t.Fatalf("XrayVersion = %v, want previous version", snapshot.XrayVersion)
	}
	logs := logBuffer.String()
	if !strings.Contains(logs, "Internal Status") || !strings.Contains(logs, "true") {
		t.Fatalf("logs did not report live internal status:\n%s", logs)
	}
}

func TestStartXrayFailureMarksOfflineWhenCoreStopped(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	versionValue := "25.1.1"
	runtimeState.SetXrayStarted(&versionValue, map[string]any{}, state.Hashes{EmptyConfig: "old"})
	controller := XrayController{
		state:    runtimeState,
		logger:   slog.Default(),
		core:     &fakeCore{started: true, version: "25.1.1", startErr: errors.New("start boom")},
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
		t.Fatalf("running=%v cached=%v, want both false after core stopped", snapshot.XrayRunning, snapshot.XrayInternalStatusCached)
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
	if core.StartCalls() != 0 {
		t.Fatalf("startCalls = %d, want 0", core.StartCalls())
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

	if core.StartCalls() != 1 {
		t.Fatalf("startCalls = %d, want 1", core.StartCalls())
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

func TestStartXrayRejectsConcurrentRequestWhileStartInProgress(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{version: "25.1.1", startBlock: make(chan struct{})}
	controller := XrayController{
		state:    runtimeState,
		logger:   slog.Default(),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
			"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h1","inbounds":[]}},
			"xrayConfig":{}
		}`))
		controller.Start(ctx)
		if rec.Code != http.StatusOK {
			t.Errorf("first status = %d, want 200", rec.Code)
		}
	}()

	waitForStartCall(t, core, 1)

	secondRec := httptest.NewRecorder()
	secondCtx, _ := gin.CreateTestContext(secondRec)
	secondCtx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":false,"hashes":{"emptyConfig":"h2","inbounds":[]}},
		"xrayConfig":{}
	}`))
	controller.Start(secondCtx)

	var secondBody struct {
		Response struct {
			IsStarted bool    `json:"isStarted"`
			Error     *string `json:"error"`
		} `json:"response"`
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("unmarshal second response: %v; body=%s", err, secondRec.Body.String())
	}
	if secondBody.Response.IsStarted || secondBody.Response.Error == nil || *secondBody.Response.Error != "Request already in progress" {
		t.Fatalf("second response = %s", secondRec.Body.String())
	}
	if core.StartCalls() != 1 {
		t.Fatalf("startCalls = %d, want 1 while first request is blocked", core.StartCalls())
	}

	close(core.startBlock)
	select {
	case <-firstDone:
	case <-time.After(5 * time.Second):
		t.Fatalf("first start did not finish")
	}

	thirdRec := httptest.NewRecorder()
	thirdCtx, _ := gin.CreateTestContext(thirdRec)
	thirdCtx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
		"internals":{"forceRestart":true,"hashes":{"emptyConfig":"h3","inbounds":[]}},
		"xrayConfig":{}
	}`))
	controller.Start(thirdCtx)
	if thirdRec.Code != http.StatusOK {
		t.Fatalf("third status = %d, want 200", thirdRec.Code)
	}
	if core.StartCalls() != 2 {
		t.Fatalf("startCalls = %d, want 2 after first request released", core.StartCalls())
	}
}

func TestStartXrayReleasesInProgressAfterFailure(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{version: "25.1.1", startErr: errors.New("boom")}
	controller := XrayController{
		state:    runtimeState,
		logger:   slog.Default(),
		core:     core,
		builder:  xray.ConfigBuilder{},
		snapshot: fixedSystemSnapshotter(),
	}

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{
			"internals":{"forceRestart":true,"hashes":{"emptyConfig":"h1","inbounds":[]}},
			"xrayConfig":{}
		}`))
		controller.Start(ctx)
		var body struct {
			Response struct {
				Error *string `json:"error"`
			} `json:"response"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("iteration %d unmarshal response: %v; body=%s", i, err, rec.Body.String())
		}
		if body.Response.Error == nil || *body.Response.Error != "boom" {
			t.Fatalf("iteration %d response = %s", i, rec.Body.String())
		}
	}
	if core.StartCalls() != 2 {
		t.Fatalf("startCalls = %d, want 2", core.StartCalls())
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
	started               bool
	version               string
	startErr              error
	startErrLeavesRunning bool
	healthErr             error
	startCalls            int
	startBlock            chan struct{}
	config                []byte
	handler               xray.HandlerClient
	stats                 xray.StatsClient
	routing               xray.RoutingClient
	mu                    sync.Mutex
}

func (f *fakeCore) Start(ctx context.Context, config []byte) error {
	f.mu.Lock()
	f.startCalls++
	f.mu.Unlock()
	if f.startBlock != nil {
		select {
		case <-f.startBlock:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.startErr != nil {
		f.mu.Lock()
		if !f.startErrLeavesRunning {
			f.started = false
		}
		f.mu.Unlock()
		return f.startErr
	}
	f.mu.Lock()
	f.started = true
	f.healthErr = nil
	f.config = append([]byte(nil), config...)
	f.mu.Unlock()
	return nil
}

func (f *fakeCore) Stop(ctx context.Context) error {
	f.started = false
	return nil
}

func (f *fakeCore) IsRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
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

func (f *fakeCore) StartCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCalls
}

func waitForStartCall(t *testing.T, core *fakeCore, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if core.StartCalls() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("startCalls = %d, want at least %d", core.StartCalls(), want)
}

func testLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
