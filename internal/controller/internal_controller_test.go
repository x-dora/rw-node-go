package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/state"
)

func TestInternalWebhookCollectsReportsWhenTorrentBlockerEnabled(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	runtimeState.SyncTorrentBlockerPlugin("uuid", "torrent-blocker", map[string]any{}, state.TorrentBlockerConfig{
		Enabled:       true,
		BlockDuration: 30,
	})
	ctrl := InternalController{state: runtimeState, logger: slog.Default()}

	runInternalWebhookRequest(t, ctrl.Webhook, webhookJSON(`"udp:[2001:db8::1]:443"`, `"user-1"`))
	reports := runtimeState.FlushTorrentBlockerReports()
	if len(reports) != 1 || reports[0].ActionReport.IP != "2001:db8::1" || reports[0].ActionReport.UserID != "user-1" {
		t.Fatalf("reports = %#v", reports)
	}
}

func TestInternalWebhookIgnoresInvalidDisabledAndIgnored(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	ctrl := InternalController{state: runtimeState, logger: slog.Default()}

	runInternalWebhookRequest(t, ctrl.Webhook, webhookJSON(`"tcp:203.0.113.1:443"`, `"user-1"`))
	if got := runtimeState.TorrentBlockerReportsCount(); got != 0 {
		t.Fatalf("reports count disabled = %d", got)
	}

	runtimeState.SyncTorrentBlockerPlugin("uuid", "torrent-blocker", map[string]any{}, state.TorrentBlockerConfig{
		Enabled:       true,
		BlockDuration: 30,
		IgnoredIPs:    []string{"203.0.113.1"},
		IgnoredUsers:  []string{"ignored-user"},
	})
	runInternalWebhookRequest(t, ctrl.Webhook, webhookJSON(`"tcp:203.0.113.1:443"`, `"user-1"`))
	runInternalWebhookRequest(t, ctrl.Webhook, webhookJSON(`"tcp:203.0.113.2:443"`, `"ignored-user"`))
	runInternalWebhookRequest(t, ctrl.Webhook, webhookJSON(`null`, `"user-1"`))
	runInternalWebhookRequest(t, ctrl.Webhook, `{`)
	if got := runtimeState.TorrentBlockerReportsCount(); got != 0 {
		t.Fatalf("reports count ignored = %d", got)
	}
}

func runInternalWebhookRequest(t *testing.T, handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/internal/webhook", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var accepted map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("unmarshal webhook response: %v; body=%s", err, rec.Body.String())
	}
	return rec
}

func webhookJSON(source string, email string) string {
	return `{
		"email":` + email + `,
		"level":0,
		"protocol":"vless",
		"network":"tcp",
		"source":` + source + `,
		"destination":"tracker.example:443",
		"routeTarget":null,
		"originalTarget":null,
		"inboundTag":"VLESS_INBOUND",
		"inboundName":null,
		"inboundLocal":null,
		"outboundTag":"DIRECT",
		"ts":1710000000
	}`
}
