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

func TestPluginSyncIsAdapterOnly(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{started: true}
	ctrl := PluginController{state: runtimeState, logger: slog.Default(), core: core}

	rec := runPluginRequest(t, ctrl.Sync, enabledTorrentPluginJSON())
	assertAccepted(t, rec.Body.String(), true)
	if !core.started {
		t.Fatalf("plugin sync stopped xray")
	}
	if runtimeState.TorrentBlockerReportsCount() != 0 {
		t.Fatalf("plugin sync created runtime state")
	}
}

func TestPluginSyncInvalidJSONRejected(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := PluginController{state: state.NewRuntimeState(), logger: slog.Default()}

	rec := runPluginRequest(t, ctrl.Sync, `{`)
	assertAccepted(t, rec.Body.String(), false)
}

func TestPluginSyncNullPluginIsAcceptedAdapterOnly(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{started: true}
	ctrl := PluginController{state: runtimeState, logger: slog.Default(), core: core}

	rec := runPluginRequest(t, ctrl.Sync, `{"plugin":null}`)

	assertAccepted(t, rec.Body.String(), true)
	if !core.started {
		t.Fatalf("plugin null sync stopped xray")
	}
	if runtimeState.TorrentBlockerReportsCount() != 0 {
		t.Fatalf("plugin null sync created runtime state")
	}
}

func TestCollectTorrentBlockerReportsReturnsEmpty(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := PluginController{state: state.NewRuntimeState(), logger: slog.Default()}

	rec := runPluginRequest(t, ctrl.CollectTorrentBlockerReports, ``)
	var decoded struct {
		Response struct {
			Reports []any `json:"reports"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal collect: %v; body=%s", err, rec.Body.String())
	}
	if len(decoded.Response.Reports) != 0 {
		t.Fatalf("reports = %#v", decoded.Response.Reports)
	}
}

func runPluginRequest(t *testing.T, handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/plugin/test", strings.NewReader(body))
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	handler(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	return rec
}

func assertAccepted(t *testing.T, body string, want bool) {
	t.Helper()
	var decoded struct {
		Response struct {
			Accepted bool `json:"accepted"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal accepted: %v; body=%s", err, body)
	}
	if decoded.Response.Accepted != want {
		t.Fatalf("accepted = %v, want %v; body=%s", decoded.Response.Accepted, want, body)
	}
}

func enabledTorrentPluginJSON() string {
	return `{
		"plugin":{
			"uuid":"44444444-4444-4444-8444-444444444444",
			"name":"torrent-blocker",
			"config":{
				"torrentBlocker":{"enabled":true}
			}
		}
	}`
}
