package controller

import (
	"context"
	"encoding/json"
	"errors"
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

func TestHealthcheckStubIncludesNodeVersion(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{}
	controller := XrayController{
		state:   runtimeState,
		logger:  slog.Default(),
		core:    core,
		builder: xray.ConfigBuilder{XTLSAPIPort: 61000},
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/node/xray/healthcheck", nil)

	controller.Healthcheck(ctx)

	var body struct {
		Response struct {
			IsAlive     bool   `json:"isAlive"`
			NodeVersion string `json:"nodeVersion"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Response.IsAlive {
		t.Fatalf("IsAlive = true, want false")
	}
	if body.Response.NodeVersion != version.Version {
		t.Fatalf("NodeVersion = %q, want %q", body.Response.NodeVersion, version.Version)
	}
}

func TestStartXrayStartsCore(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{version: "25.1.1"}
	controller := XrayController{
		state:   runtimeState,
		logger:  slog.Default(),
		core:    core,
		builder: xray.ConfigBuilder{XTLSAPIPort: 61000},
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
	if !core.started {
		t.Fatalf("core was not started")
	}
}

func TestStartXrayReturnsErrorWhenCoreFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	controller := XrayController{
		state:   runtimeState,
		logger:  slog.Default(),
		core:    &fakeCore{startErr: errors.New("boom")},
		builder: xray.ConfigBuilder{XTLSAPIPort: 61000},
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
}

func TestStartXraySkipsRestartWhenHashesUnchanged(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	hashes := state.Hashes{EmptyConfig: "h1"}
	versionValue := "25.1.1"
	runtimeState.SetXrayStarted(&versionValue, map[string]any{}, hashes)
	core := &fakeCore{started: true, version: "25.1.1"}
	controller := XrayController{
		state:   runtimeState,
		logger:  slog.Default(),
		core:    core,
		builder: xray.ConfigBuilder{XTLSAPIPort: 61000},
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
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !body.Response.IsStarted || body.Response.Error != nil {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestHandlerStubEnvelope(t *testing.T) {
	t.Skip("handler stubs are covered by route envelope tests")
}

type fakeCore struct {
	started  bool
	version  string
	startErr error
}

func (f *fakeCore) Start(ctx context.Context, config []byte) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = true
	return nil
}

func (f *fakeCore) Stop(ctx context.Context) error {
	f.started = false
	return nil
}

func (f *fakeCore) IsRunning() bool {
	return f.started
}

func (f *fakeCore) Version(ctx context.Context) (string, error) {
	return f.version, nil
}

func (f *fakeCore) Handler() xray.HandlerClient {
	return nil
}

func (f *fakeCore) Stats() xray.StatsClient {
	return nil
}

func (f *fakeCore) Routing() xray.RoutingClient {
	return nil
}
