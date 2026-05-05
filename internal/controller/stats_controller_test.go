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
	"github.com/x-dora/rw-node-go/internal/xray"
	statscommand "github.com/xtls/xray-core/app/stats/command"
)

func TestStatsControllerReturnsRealStatsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	stats := &recordingStatsClient{
		sys: xray.SysStats{NumGoroutine: 2, TotalAlloc: 3, PauseTotalNs: 4},
		users: []xray.UserTrafficStats{
			{Username: "user-1", Uplink: 10, Downlink: 20},
		},
		inbound:  xray.InboundTrafficStats{Inbound: "VLESS_INBOUND", Uplink: 30, Downlink: 40},
		outbound: xray.OutboundTrafficStats{Outbound: "DIRECT", Uplink: 50, Downlink: 60},
		inbounds: []xray.InboundTrafficStats{
			{Inbound: "VLESS_INBOUND", Uplink: 70, Downlink: 80},
		},
		outbounds: []xray.OutboundTrafficStats{
			{Outbound: "DIRECT", Uplink: 90, Downlink: 100},
		},
	}
	ctrl := StatsController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{started: true, stats: stats}}

	systemRec := runStatsRequest(t, ctrl.GetSystemStats, http.MethodGet, nil)
	var systemBody struct {
		Response struct {
			XrayInfo *struct {
				NumGoroutine uint32 `json:"numGoroutine"`
				TotalAlloc   uint64 `json:"totalAlloc"`
				PauseTotalNs uint64 `json:"pauseTotalNs"`
			} `json:"xrayInfo"`
			Plugins struct {
				TorrentBlocker struct {
					ReportsCount int `json:"reportsCount"`
				} `json:"torrentBlocker"`
			} `json:"plugins"`
			System struct {
				Stats struct {
					LoadAvg []float64 `json:"loadAvg"`
				} `json:"stats"`
			} `json:"system"`
		} `json:"response"`
	}
	decodeResponse(t, systemRec, &systemBody)
	if systemBody.Response.XrayInfo == nil || systemBody.Response.XrayInfo.NumGoroutine != 2 || systemBody.Response.XrayInfo.TotalAlloc != 3 || systemBody.Response.XrayInfo.PauseTotalNs != 4 {
		t.Fatalf("system body = %s", systemRec.Body.String())
	}
	if systemBody.Response.Plugins.TorrentBlocker.ReportsCount != 0 || len(systemBody.Response.System.Stats.LoadAvg) != 3 {
		t.Fatalf("system body = %s", systemRec.Body.String())
	}

	usersRec := runStatsRequest(t, ctrl.GetUsersStats, http.MethodPost, `{"reset":true}`)
	var usersBody struct {
		Response struct {
			Users []struct {
				Username string `json:"username"`
				Uplink   int64  `json:"uplink"`
				Downlink int64  `json:"downlink"`
			} `json:"users"`
		} `json:"response"`
	}
	decodeResponse(t, usersRec, &usersBody)
	if len(usersBody.Response.Users) != 1 || usersBody.Response.Users[0].Username != "user-1" || usersBody.Response.Users[0].Uplink != 10 {
		t.Fatalf("users body = %s", usersRec.Body.String())
	}

	inboundRec := runStatsRequest(t, ctrl.GetInboundStats, http.MethodPost, `{"tag":"VLESS_INBOUND","reset":true}`)
	var inboundBody struct {
		Response struct {
			Inbound  string `json:"inbound"`
			Uplink   int64  `json:"uplink"`
			Downlink int64  `json:"downlink"`
		} `json:"response"`
	}
	decodeResponse(t, inboundRec, &inboundBody)
	if inboundBody.Response.Inbound != "VLESS_INBOUND" || inboundBody.Response.Uplink != 30 || !stats.inboundReset {
		t.Fatalf("inbound body=%s reset=%v", inboundRec.Body.String(), stats.inboundReset)
	}

	outboundRec := runStatsRequest(t, ctrl.GetOutboundStats, http.MethodPost, `{"tag":"DIRECT","reset":false}`)
	var outboundBody struct {
		Response struct {
			Outbound string `json:"outbound"`
			Uplink   int64  `json:"uplink"`
			Downlink int64  `json:"downlink"`
		} `json:"response"`
	}
	decodeResponse(t, outboundRec, &outboundBody)
	if outboundBody.Response.Outbound != "DIRECT" || outboundBody.Response.Downlink != 60 || stats.outboundReset {
		t.Fatalf("outbound body=%s reset=%v", outboundRec.Body.String(), stats.outboundReset)
	}

	combinedRec := runStatsRequest(t, ctrl.GetCombinedStats, http.MethodPost, `{"reset":true}`)
	var combinedBody struct {
		Response struct {
			Inbounds  []struct{ Inbound string }  `json:"inbounds"`
			Outbounds []struct{ Outbound string } `json:"outbounds"`
		} `json:"response"`
	}
	decodeResponse(t, combinedRec, &combinedBody)
	if len(combinedBody.Response.Inbounds) != 1 || combinedBody.Response.Inbounds[0].Inbound != "VLESS_INBOUND" || len(combinedBody.Response.Outbounds) != 1 || combinedBody.Response.Outbounds[0].Outbound != "DIRECT" {
		t.Fatalf("combined body = %s", combinedRec.Body.String())
	}
	if !stats.allInboundReset || !stats.allOutboundReset {
		t.Fatalf("combined did not pass reset")
	}
}

func TestStatsControllerDegradesWhenCoreUnavailable(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := StatsController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{}}

	systemRec := runStatsRequest(t, ctrl.GetSystemStats, http.MethodGet, nil)
	var systemBody struct {
		Response struct {
			XrayInfo *json.RawMessage `json:"xrayInfo"`
		} `json:"response"`
	}
	decodeResponse(t, systemRec, &systemBody)
	if systemBody.Response.XrayInfo != nil {
		t.Fatalf("xrayInfo = %v, want nil", systemBody.Response.XrayInfo)
	}

	usersRec := runStatsRequest(t, ctrl.GetUsersStats, http.MethodPost, `{"reset":true}`)
	var usersBody struct {
		Response struct {
			Users []any `json:"users"`
		} `json:"response"`
	}
	decodeResponse(t, usersRec, &usersBody)
	if len(usersBody.Response.Users) != 0 {
		t.Fatalf("users body = %s", usersRec.Body.String())
	}

	inboundRec := runStatsRequest(t, ctrl.GetInboundStats, http.MethodPost, `{"tag":"VLESS_INBOUND","reset":true}`)
	var inboundBody struct {
		Response struct {
			Inbound  string `json:"inbound"`
			Uplink   int64  `json:"uplink"`
			Downlink int64  `json:"downlink"`
		} `json:"response"`
	}
	decodeResponse(t, inboundRec, &inboundBody)
	if inboundBody.Response.Inbound != "VLESS_INBOUND" || inboundBody.Response.Uplink != 0 || inboundBody.Response.Downlink != 0 {
		t.Fatalf("inbound body = %s", inboundRec.Body.String())
	}
}

func TestStatsControllerDegradesOnBindAndClientErrors(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := StatsController{
		state:  state.NewRuntimeState(),
		logger: slog.Default(),
		core:   &fakeCore{started: true, stats: &recordingStatsClient{err: errors.New("boom")}},
	}

	badRec := runStatsRequest(t, ctrl.GetUsersStats, http.MethodPost, `{`)
	var usersBody struct {
		Response struct {
			Users []any `json:"users"`
		} `json:"response"`
	}
	decodeResponse(t, badRec, &usersBody)
	if badRec.Code != http.StatusOK || len(usersBody.Response.Users) != 0 {
		t.Fatalf("bad request response = %s", badRec.Body.String())
	}

	allRec := runStatsRequest(t, ctrl.GetAllInboundsStats, http.MethodPost, `{"reset":true}`)
	var allBody struct {
		Response struct {
			Inbounds []any `json:"inbounds"`
		} `json:"response"`
	}
	decodeResponse(t, allRec, &allBody)
	if len(allBody.Response.Inbounds) != 0 {
		t.Fatalf("all inbounds body = %s", allRec.Body.String())
	}
}

func TestStatsControllerOnlineAndIPRemainStubs(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := StatsController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{started: true}}

	onlineRec := runStatsRequest(t, ctrl.GetUserOnlineStatus, http.MethodPost, `{"username":"user-1"}`)
	var onlineBody struct {
		Response struct {
			IsOnline bool `json:"isOnline"`
		} `json:"response"`
	}
	decodeResponse(t, onlineRec, &onlineBody)
	if onlineBody.Response.IsOnline {
		t.Fatalf("online body = %s", onlineRec.Body.String())
	}

	ipRec := runStatsRequest(t, ctrl.GetUserIPList, http.MethodPost, `{"userId":"user-1"}`)
	var ipBody struct {
		Response struct {
			IPs []any `json:"ips"`
		} `json:"response"`
	}
	decodeResponse(t, ipRec, &ipBody)
	if len(ipBody.Response.IPs) != 0 {
		t.Fatalf("ip body = %s", ipRec.Body.String())
	}

	usersIPRec := runStatsRequest(t, ctrl.GetUsersIPList, http.MethodGet, nil)
	var usersIPBody struct {
		Response struct {
			Users []any `json:"users"`
		} `json:"response"`
	}
	decodeResponse(t, usersIPRec, &usersIPBody)
	if len(usersIPBody.Response.Users) != 0 {
		t.Fatalf("users ip body = %s", usersIPRec.Body.String())
	}
}

func runStatsRequest(t *testing.T, handler gin.HandlerFunc, method string, body any) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	var reader *strings.Reader
	if value, ok := body.(string); ok {
		reader = strings.NewReader(value)
	} else {
		reader = strings.NewReader("")
	}
	ctx.Request = httptest.NewRequest(method, "/node/stats/test", reader)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	handler(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	return rec
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
}

type recordingStatsClient struct {
	sys              xray.SysStats
	users            []xray.UserTrafficStats
	inbound          xray.InboundTrafficStats
	outbound         xray.OutboundTrafficStats
	inbounds         []xray.InboundTrafficStats
	outbounds        []xray.OutboundTrafficStats
	err              error
	inboundReset     bool
	outboundReset    bool
	allInboundReset  bool
	allOutboundReset bool
}

func (c *recordingStatsClient) Ping(ctx context.Context) error {
	return c.err
}

func (c *recordingStatsClient) SysStats(ctx context.Context) (xray.SysStats, error) {
	return c.sys, c.err
}

func (c *recordingStatsClient) UsersStats(ctx context.Context, reset bool) ([]xray.UserTrafficStats, error) {
	return c.users, c.err
}

func (c *recordingStatsClient) InboundStats(ctx context.Context, tag string, reset bool) (xray.InboundTrafficStats, error) {
	c.inboundReset = reset
	return c.inbound, c.err
}

func (c *recordingStatsClient) OutboundStats(ctx context.Context, tag string, reset bool) (xray.OutboundTrafficStats, error) {
	c.outboundReset = reset
	return c.outbound, c.err
}

func (c *recordingStatsClient) AllInboundStats(ctx context.Context, reset bool) ([]xray.InboundTrafficStats, error) {
	c.allInboundReset = reset
	return c.inbounds, c.err
}

func (c *recordingStatsClient) AllOutboundStats(ctx context.Context, reset bool) ([]xray.OutboundTrafficStats, error) {
	c.allOutboundReset = reset
	return c.outbounds, c.err
}

func (c *recordingStatsClient) Raw() statscommand.StatsServiceClient {
	return nil
}
