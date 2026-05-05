package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/controller"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/testkit"
	"github.com/x-dora/rw-node-go/internal/xray"
	handlercommand "github.com/xtls/xray-core/app/proxyman/command"
	statscommand "github.com/xtls/xray-core/app/stats/command"
)

func TestOfficialPanelRoutesAreRegistered(t *testing.T) {
	fixture := testkit.LoadPanelAPIFixture(t)
	router := newTestRouter(t)

	for _, route := range fixture.Routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			req := httptest.NewRequest(route.Method, route.Path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("route returned 404")
			}
			if rec.Code == http.StatusMethodNotAllowed {
				t.Fatalf("route returned 405")
			}
		})
	}
}

func TestInternalRoutesAreRegistered(t *testing.T) {
	router := newTestRouter(t)
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/internal/get-config"},
		{http.MethodPost, "/internal/webhook"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("route returned 404")
			}
			if rec.Code == http.StatusMethodNotAllowed {
				t.Fatalf("route returned 405")
			}
		})
	}
}

func TestStubResponsesMatchOfficialEmptyShape(t *testing.T) {
	fixture := testkit.LoadPanelAPIFixture(t)
	router := newTestRouter(t)

	for _, route := range fixture.Routes {
		if route.Status == "implemented" || route.Name == "xray.start" || isPartialStatsRoute(route.Name) {
			continue
		}
		t.Run(route.Name, func(t *testing.T) {
			var body *bytes.Reader
			if len(route.Request) > 0 {
				body = bytes.NewReader(route.Request)
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(route.Method, route.Path, body)
			if len(route.Request) > 0 {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			assertFixtureResponseShape(t, rec.Body.Bytes(), route.Response)
		})
	}
}

func isPartialStatsRoute(name string) bool {
	switch name {
	case "stats.get-system-stats",
		"stats.get-users-stats",
		"stats.get-inbound-stats",
		"stats.get-outbound-stats",
		"stats.get-all-inbounds-stats",
		"stats.get-all-outbounds-stats",
		"stats.get-combined-stats":
		return true
	default:
		return false
	}
}

func assertFixtureResponseShape(t *testing.T, got []byte, want json.RawMessage) {
	t.Helper()
	testkit.AssertCanonicalJSONEqual(t, got, want)
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()

	runtimeState := state.NewRuntimeState()
	logger := slog.New(slog.NewTextHandler(testWriter{t: t}, nil))
	controllers := controller.NewRegistryWithXray(
		runtimeState,
		logger,
		&routerFakeCore{},
		xray.ConfigBuilder{XTLSAPIPort: 61000},
	)
	return httpapi.NewRouter(config.Config{RequestBodyLimitBytes: 1 << 20}, httpapi.Handlers{
		Xray:     controllers.Xray,
		Handler:  controllers.Handler,
		Stats:    controllers.Stats,
		Vision:   controllers.Vision,
		Plugin:   controllers.Plugin,
		Internal: controllers.Internal,
	}, logger)
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	return len(p), nil
}

type routerFakeCore struct{}

func (f *routerFakeCore) Start(ctx context.Context, config []byte) error {
	return nil
}

func (f *routerFakeCore) Stop(ctx context.Context) error {
	return nil
}

func (f *routerFakeCore) IsRunning() bool {
	return true
}

func (f *routerFakeCore) Health(ctx context.Context) error {
	return nil
}

func (f *routerFakeCore) Version(ctx context.Context) (string, error) {
	return "25.1.1", nil
}

func (f *routerFakeCore) Handler() xray.HandlerClient {
	return routerFakeHandler{}
}

func (f *routerFakeCore) Stats() xray.StatsClient {
	return routerFakeStats{}
}

func (f *routerFakeCore) Routing() xray.RoutingClient {
	return nil
}

type routerFakeHandler struct{}

func (routerFakeHandler) AddUser(ctx context.Context, spec xray.UserSpec) error {
	return nil
}

func (routerFakeHandler) RemoveUser(ctx context.Context, tag string, username string) error {
	return nil
}

func (routerFakeHandler) GetInboundUsers(ctx context.Context, tag string) ([]xray.InboundUser, error) {
	return []xray.InboundUser{}, nil
}

func (routerFakeHandler) GetInboundUsersCount(ctx context.Context, tag string) (int, error) {
	return 0, nil
}

func (routerFakeHandler) Raw() handlercommand.HandlerServiceClient {
	return nil
}

type routerFakeStats struct{}

func (routerFakeStats) Ping(ctx context.Context) error {
	return nil
}

func (routerFakeStats) SysStats(ctx context.Context) (xray.SysStats, error) {
	return xray.SysStats{}, nil
}

func (routerFakeStats) UsersStats(ctx context.Context, reset bool) ([]xray.UserTrafficStats, error) {
	return []xray.UserTrafficStats{}, nil
}

func (routerFakeStats) UserOnlineStatus(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (routerFakeStats) UserIPList(ctx context.Context, username string, reset bool) ([]xray.IPLastSeen, error) {
	return []xray.IPLastSeen{}, nil
}

func (routerFakeStats) UsersIPList(ctx context.Context, reset bool) ([]xray.UserIPList, error) {
	return []xray.UserIPList{}, nil
}

func (routerFakeStats) InboundStats(ctx context.Context, tag string, reset bool) (xray.InboundTrafficStats, error) {
	return xray.InboundTrafficStats{Inbound: tag}, nil
}

func (routerFakeStats) OutboundStats(ctx context.Context, tag string, reset bool) (xray.OutboundTrafficStats, error) {
	return xray.OutboundTrafficStats{Outbound: tag}, nil
}

func (routerFakeStats) AllInboundStats(ctx context.Context, reset bool) ([]xray.InboundTrafficStats, error) {
	return []xray.InboundTrafficStats{}, nil
}

func (routerFakeStats) AllOutboundStats(ctx context.Context, reset bool) ([]xray.OutboundTrafficStats, error) {
	return []xray.OutboundTrafficStats{}, nil
}

func (routerFakeStats) Raw() statscommand.StatsServiceClient {
	return nil
}
