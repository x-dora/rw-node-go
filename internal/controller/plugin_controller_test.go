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

func TestPluginSyncNullCleansActivePlugin(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{started: true}
	ctrl := PluginController{state: runtimeState, logger: slog.Default(), core: core}

	rec := runPluginRequest(t, ctrl.Sync, `{"plugin":null}`)
	assertAccepted(t, rec.Body.String(), false)

	rec = runPluginRequest(t, ctrl.Sync, enabledTorrentPluginJSON())
	assertAccepted(t, rec.Body.String(), true)
	if !runtimeState.HasActivePlugin() {
		t.Fatalf("active plugin was not set")
	}

	rec = runPluginRequest(t, ctrl.Sync, `{"plugin":null}`)
	assertAccepted(t, rec.Body.String(), true)
	if runtimeState.HasActivePlugin() {
		t.Fatalf("active plugin was not cleaned")
	}
	if core.started || runtimeState.Snapshot().XrayInternalStatusCached {
		t.Fatalf("xray was not stopped after plugin cleanup")
	}
}

func TestPluginSyncStoresTorrentBlockerState(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	ctrl := PluginController{state: runtimeState, logger: slog.Default()}

	rec := runPluginRequest(t, ctrl.Sync, enabledTorrentPluginJSON())
	assertAccepted(t, rec.Body.String(), true)

	snapshot := runtimeState.TorrentBlockerSnapshot()
	if !snapshot.Enabled || snapshot.BlockDuration != 60 {
		t.Fatalf("torrent snapshot = %#v", snapshot)
	}
	if !contains(snapshot.IgnoredIPs, "198.51.100.1") || !contains(snapshot.IgnoredIPs, "203.0.113.10") {
		t.Fatalf("ignored IPs = %#v", snapshot.IgnoredIPs)
	}
	if !contains(snapshot.IgnoredUsers, "user-ignored") || !contains(snapshot.IncludeRuleTags, "DIRECT_RULE") {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestPluginSyncInvalidConfigResetsState(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{started: true}
	ctrl := PluginController{state: runtimeState, logger: slog.Default(), core: core}

	assertAccepted(t, runPluginRequest(t, ctrl.Sync, enabledTorrentPluginJSON()).Body.String(), true)
	assertAccepted(t, runPluginRequest(t, ctrl.Sync, `{
		"plugin":{
			"uuid":"44444444-4444-4444-8444-444444444444",
			"name":"torrent-blocker",
			"config":{"torrentBlocker":{"enabled":true,"ignoreLists":{"ip":[],"userId":[]}}}
		}
	}`).Body.String(), false)

	if runtimeState.HasActivePlugin() || runtimeState.TorrentBlockerSnapshot().Enabled {
		t.Fatalf("invalid sync did not reset plugin state")
	}
	if core.started {
		t.Fatalf("xray was not stopped after invalid plugin sync")
	}
}

func TestPluginSyncTorrentBlockerModeChangeStopsXray(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	core := &fakeCore{started: true}
	ctrl := PluginController{state: runtimeState, logger: slog.Default(), core: core}

	assertAccepted(t, runPluginRequest(t, ctrl.Sync, enabledTorrentPluginJSON()).Body.String(), true)
	if core.started {
		t.Fatalf("xray was not stopped after enabling torrent blocker")
	}

	core.started = true
	assertAccepted(t, runPluginRequest(t, ctrl.Sync, disabledTorrentPluginJSON()).Body.String(), true)
	if core.started {
		t.Fatalf("xray was not stopped after disabling torrent blocker")
	}
}

func TestCollectTorrentBlockerReportsFlushesQueue(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	ctrl := PluginController{state: runtimeState, logger: slog.Default()}
	internal := InternalController{state: runtimeState, logger: slog.Default()}

	assertAccepted(t, runPluginRequest(t, ctrl.Sync, enabledTorrentPluginJSON()).Body.String(), true)
	runInternalWebhookRequest(t, internal.Webhook, webhookJSON(`"tcp:203.0.113.44:443"`, `"user-1"`))

	first := runPluginRequest(t, ctrl.CollectTorrentBlockerReports, ``)
	var firstBody struct {
		Response struct {
			Reports []struct {
				ActionReport struct {
					Blocked       bool   `json:"blocked"`
					IP            string `json:"ip"`
					BlockDuration int    `json:"blockDuration"`
					UserID        string `json:"userId"`
				} `json:"actionReport"`
				XrayReport struct {
					Email *string `json:"email"`
				} `json:"xrayReport"`
			} `json:"reports"`
		} `json:"response"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("unmarshal collect: %v; body=%s", err, first.Body.String())
	}
	if len(firstBody.Response.Reports) != 1 ||
		firstBody.Response.Reports[0].ActionReport.Blocked ||
		firstBody.Response.Reports[0].ActionReport.IP != "203.0.113.44" ||
		firstBody.Response.Reports[0].ActionReport.BlockDuration != 60 ||
		firstBody.Response.Reports[0].ActionReport.UserID != "user-1" ||
		firstBody.Response.Reports[0].XrayReport.Email == nil ||
		*firstBody.Response.Reports[0].XrayReport.Email != "user-1" {
		t.Fatalf("first collect body = %s", first.Body.String())
	}

	second := runPluginRequest(t, ctrl.CollectTorrentBlockerReports, ``)
	var secondBody struct {
		Response struct {
			Reports []any `json:"reports"`
		} `json:"response"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("unmarshal second collect: %v", err)
	}
	if len(secondBody.Response.Reports) != 0 {
		t.Fatalf("second collect body = %s", second.Body.String())
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
				"sharedLists":[{"type":"ipList","name":"trusted","items":["198.51.100.1"]}],
				"torrentBlocker":{
					"enabled":true,
					"blockDuration":60,
					"ignoreLists":{"ip":["ext:trusted","203.0.113.10"],"userId":["user-ignored"]},
					"includeRuleTags":["DIRECT_RULE"]
				}
			}
		}
	}`
}

func disabledTorrentPluginJSON() string {
	return `{
		"plugin":{
			"uuid":"44444444-4444-4444-8444-444444444444",
			"name":"torrent-blocker",
			"config":{
				"sharedLists":[],
				"torrentBlocker":{"enabled":false}
			}
		}
	}`
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
