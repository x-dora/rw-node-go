package httpapi_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/controller"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

func TestPlannedRoutesAreRegistered(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/node/xray/start"},
		{http.MethodGet, "/node/xray/stop"},
		{http.MethodGet, "/node/xray/healthcheck"},
		{http.MethodPost, "/node/handler/add-user"},
		{http.MethodPost, "/node/handler/add-users"},
		{http.MethodPost, "/node/handler/remove-user"},
		{http.MethodPost, "/node/handler/remove-users"},
		{http.MethodPost, "/node/handler/get-inbound-users"},
		{http.MethodPost, "/node/handler/get-inbound-users-count"},
		{http.MethodPost, "/node/handler/drop-users-connections"},
		{http.MethodPost, "/node/handler/drop-ips"},
		{http.MethodGet, "/node/stats/get-system-stats"},
		{http.MethodPost, "/node/stats/get-users-stats"},
		{http.MethodPost, "/node/stats/get-user-online-status"},
		{http.MethodPost, "/node/stats/get-user-ip-list"},
		{http.MethodGet, "/node/stats/get-users-ip-list"},
		{http.MethodPost, "/node/stats/get-inbound-stats"},
		{http.MethodPost, "/node/stats/get-outbound-stats"},
		{http.MethodPost, "/node/stats/get-all-inbounds-stats"},
		{http.MethodPost, "/node/stats/get-all-outbounds-stats"},
		{http.MethodPost, "/node/stats/get-combined-stats"},
		{http.MethodPost, "/vision/block-ip"},
		{http.MethodPost, "/vision/unblock-ip"},
		{http.MethodPost, "/node/plugin/sync"},
		{http.MethodPost, "/node/plugin/torrent-blocker/collect"},
		{http.MethodPost, "/node/plugin/nftables/block-ips"},
		{http.MethodPost, "/node/plugin/nftables/unblock-ips"},
		{http.MethodPost, "/node/plugin/nftables/recreate-tables"},
		{http.MethodGet, "/internal/get-config"},
		{http.MethodPost, "/internal/webhook"},
	}

	runtimeState := state.NewRuntimeState()
	logger := slog.New(slog.NewTextHandler(testWriter{t: t}, nil))
	controllers := controller.NewRegistry(runtimeState, logger)
	router := httpapi.NewRouter(config.Config{RequestBodyLimitBytes: 1 << 20}, httpapi.Handlers{
		Xray:     controllers.Xray,
		Handler:  controllers.Handler,
		Stats:    controllers.Stats,
		Vision:   controllers.Vision,
		Plugin:   controllers.Plugin,
		Internal: controllers.Internal,
	}, logger)

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

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	return len(p), nil
}
