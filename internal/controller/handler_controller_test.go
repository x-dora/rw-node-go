package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/system"
	"github.com/x-dora/rw-node-go/internal/xray"
)

func TestHandlerAddUserRemovesKnownInboundsThenAddsUsers(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	runtimeState.AddKnownInboundTags("OLD_INBOUND")
	runtimeState.AddUserToInbound("OLD_INBOUND", "22222222-2222-4222-8222-222222222222")
	handler := &recordingHandlerClient{}
	ctrl := HandlerController{state: runtimeState, logger: slog.Default(), core: &fakeCore{started: true, handler: handler}}

	rec := runHandlerRequest(t, ctrl.AddUser, `{
		"data":[
			{"type":"vless","tag":"VLESS_INBOUND","username":"user-1","uuid":"11111111-1111-4111-8111-111111111111","flow":"xtls-rprx-vision"},
			{"type":"trojan","tag":"TROJAN_INBOUND","username":"user-1","password":"pw"}
		],
		"hashData":{"vlessUuid":"11111111-1111-4111-8111-111111111111","prevVlessUuid":"22222222-2222-4222-8222-222222222222"}
	}`)

	assertGenericSuccess(t, rec.Body.String(), true)
	if got := handler.removeCalls; len(got) != 3 || got[0].tag != "OLD_INBOUND" || got[1].tag != "TROJAN_INBOUND" || got[2].tag != "VLESS_INBOUND" {
		t.Fatalf("removeCalls = %#v", got)
	}
	if len(handler.addSpecs) != 2 || handler.addSpecs[0].Protocol != xray.ProtocolVLESS || handler.addSpecs[1].Protocol != xray.ProtocolTrojan {
		t.Fatalf("addSpecs = %#v", handler.addSpecs)
	}
	if hashes := runtimeState.InboundUserHashes("OLD_INBOUND"); len(hashes) != 0 {
		t.Fatalf("OLD_INBOUND hashes = %#v", hashes)
	}
	if hashes := runtimeState.InboundUserHashes("VLESS_INBOUND"); len(hashes) != 1 || hashes[0] != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("VLESS_INBOUND hashes = %#v", hashes)
	}
}

func TestHandlerAddUsersUsesBulkMappings(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	handler := &recordingHandlerClient{}
	ctrl := HandlerController{state: runtimeState, logger: slog.Default(), core: &fakeCore{started: true, handler: handler}}

	rec := runHandlerRequest(t, ctrl.AddUsers, `{
		"affectedInboundTags":["SS2022_INBOUND","HYSTERIA_INBOUND"],
		"users":[{
			"inboundData":[
				{"type":"shadowsocks22","tag":"SS2022_INBOUND"},
				{"type":"hysteria","tag":"HYSTERIA_INBOUND"}
			],
			"userData":{
				"userId":"user-1",
				"hashUuid":"33333333-3333-4333-8333-333333333333",
				"vlessUuid":"11111111-1111-4111-8111-111111111111",
				"trojanPassword":"trojan-password",
				"ssPassword":"ss-password"
			}
		}]
	}`)

	assertGenericSuccess(t, rec.Body.String(), true)
	if len(handler.addSpecs) != 2 {
		t.Fatalf("addSpecs = %#v", handler.addSpecs)
	}
	if handler.addSpecs[0].Key != "c3MtcGFzc3dvcmQ=" {
		t.Fatalf("ss2022 key = %q", handler.addSpecs[0].Key)
	}
	if handler.addSpecs[1].Password != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("hysteria password = %q", handler.addSpecs[1].Password)
	}
}

func TestHandlerAddUsersTracksInboundDataTagsWhenAffectedTagsMissing(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	handler := &recordingHandlerClient{}
	ctrl := HandlerController{state: runtimeState, logger: slog.Default(), core: &fakeCore{started: true, handler: handler}}

	rec := runHandlerRequest(t, ctrl.AddUsers, `{
		"affectedInboundTags":[],
		"users":[{
			"inboundData":[{"type":"vless","tag":"VLESS_INBOUND"}],
			"userData":{
				"userId":"user-1",
				"hashUuid":"33333333-3333-4333-8333-333333333333",
				"vlessUuid":"11111111-1111-4111-8111-111111111111"
			}
		}]
	}`)

	assertGenericSuccess(t, rec.Body.String(), true)
	tags := runtimeState.KnownInboundTags()
	if len(tags) != 1 || tags[0] != "VLESS_INBOUND" {
		t.Fatalf("KnownInboundTags = %#v, want VLESS_INBOUND", tags)
	}
	if got := runtimeState.InboundProtocol("VLESS_INBOUND"); got != "vless" {
		t.Fatalf("InboundProtocol = %q, want vless", got)
	}
}

func TestHandlerRemoveUserNoKnownInboundSucceedsWithoutCore(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := HandlerController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{}}

	rec := runHandlerRequest(t, ctrl.RemoveUser, `{
		"username":"user-1",
		"hashData":{"vlessUuid":"11111111-1111-4111-8111-111111111111"}
	}`)

	assertGenericSuccess(t, rec.Body.String(), true)
}

func TestHandlerRemoveUsersRemovesAllKnownInbounds(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	runtimeState.AddKnownInboundTags("A", "B")
	runtimeState.AddUserToInbound("A", "33333333-3333-4333-8333-333333333333")
	runtimeState.AddUserToInbound("B", "33333333-3333-4333-8333-333333333333")
	handler := &recordingHandlerClient{}
	ctrl := HandlerController{state: runtimeState, logger: slog.Default(), core: &fakeCore{started: true, handler: handler}}

	rec := runHandlerRequest(t, ctrl.RemoveUsers, `{
		"users":[{"userId":"user-1","hashUuid":"33333333-3333-4333-8333-333333333333"}]
	}`)

	assertGenericSuccess(t, rec.Body.String(), true)
	if got := handler.removeCalls; len(got) != 2 || got[0].tag != "A" || got[1].tag != "B" {
		t.Fatalf("removeCalls = %#v", got)
	}
	if len(runtimeState.InboundUserHashes("A")) != 0 || len(runtimeState.InboundUserHashes("B")) != 0 {
		t.Fatalf("hashes were not removed")
	}
}

func TestHandlerQueriesReturnEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &recordingHandlerClient{
		users: []xray.InboundUser{{Username: "user-1", Level: 0, Protocol: xray.ProtocolVLESS}},
		count: 7,
	}
	ctrl := HandlerController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{started: true, handler: handler}}

	usersRec := runHandlerRequest(t, ctrl.GetInboundUsers, `{"tag":"VLESS_INBOUND"}`)
	var rawUsersBody map[string]any
	if err := json.Unmarshal(usersRec.Body.Bytes(), &rawUsersBody); err != nil {
		t.Fatalf("unmarshal raw users: %v; body=%s", err, usersRec.Body.String())
	}
	var usersBody struct {
		Response struct {
			Users []struct {
				Username string `json:"username"`
				Level    int    `json:"level"`
				Protocol string `json:"protocol"`
			} `json:"users"`
		} `json:"response"`
	}
	if err := json.Unmarshal(usersRec.Body.Bytes(), &usersBody); err != nil {
		t.Fatalf("unmarshal users: %v", err)
	}
	if len(usersBody.Response.Users) != 1 || usersBody.Response.Users[0].Username != "user-1" || usersBody.Response.Users[0].Protocol != "vless" {
		t.Fatalf("users body = %s", usersRec.Body.String())
	}
	rawUser := rawUsersBody["response"].(map[string]any)["users"].([]any)[0].(map[string]any)
	if _, ok := rawUser["email"]; ok {
		t.Fatalf("users body contains official runtime-incompatible email field: %s", usersRec.Body.String())
	}

	countRec := runHandlerRequest(t, ctrl.GetInboundUsersCount, `{"tag":"VLESS_INBOUND"}`)
	var countBody struct {
		Response struct {
			Count int `json:"count"`
		} `json:"response"`
	}
	if err := json.Unmarshal(countRec.Body.Bytes(), &countBody); err != nil {
		t.Fatalf("unmarshal count: %v", err)
	}
	if countBody.Response.Count != 7 {
		t.Fatalf("count body = %s", countRec.Body.String())
	}
}

func TestHandlerInboundUsersUseStateProtocolFallback(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	runtimeState.SetInboundProtocol("TROJAN_INBOUND", string(xray.ProtocolTrojan))
	handler := &recordingHandlerClient{
		users: []xray.InboundUser{{Username: "user-1", Level: 0}},
	}
	ctrl := HandlerController{state: runtimeState, logger: slog.Default(), core: &fakeCore{started: true, handler: handler}}

	rec := runHandlerRequest(t, ctrl.GetInboundUsers, `{"tag":"TROJAN_INBOUND"}`)
	var body struct {
		Response struct {
			Users []struct {
				Protocol string `json:"protocol"`
			} `json:"users"`
		} `json:"response"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal users: %v; body=%s", err, rec.Body.String())
	}
	if len(body.Response.Users) != 1 || body.Response.Users[0].Protocol != "trojan" {
		t.Fatalf("users body = %s", rec.Body.String())
	}
}

func TestHandlerFailureDoesNotReturnHTTP500(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := HandlerController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{}}

	rec := runHandlerRequest(t, ctrl.AddUser, `{
		"data":[{"type":"trojan","tag":"TROJAN_INBOUND","username":"user-1","password":"pw"}],
		"hashData":{"vlessUuid":"11111111-1111-4111-8111-111111111111"}
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertGenericSuccess(t, rec.Body.String(), false)
}

func TestHandlerQueryFailuresReturnOfficialErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := HandlerController{
		state:  state.NewRuntimeState(),
		logger: slog.Default(),
		core:   &fakeCore{started: true, handler: &recordingHandlerClient{err: fmt.Errorf("boom")}},
	}

	usersRec := runHandlerRequest(t, ctrl.GetInboundUsers, `{"tag":"VLESS_INBOUND"}`)
	assertOfficialErrorEnvelope(t, usersRec, http.StatusInternalServerError, "A014", "Failed to get inbound users", "/node/handler/test")

	countRec := runHandlerRequest(t, ctrl.GetInboundUsersCount, `{"tag":"VLESS_INBOUND"}`)
	assertOfficialErrorEnvelope(t, countRec, http.StatusInternalServerError, "A014", "Failed to get inbound users", "/node/handler/test")
}

func TestHandlerDropIPsUsesConnectionDropper(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	dropper := &recordingConnectionDropper{}
	ctrl := HandlerController{state: state.NewRuntimeState(), logger: slog.Default(), dropper: dropper}

	rec := runHandlerRequest(t, ctrl.DropIPs, `{"ips":["203.0.113.1","2001:db8::1"]}`)

	assertSimpleSuccess(t, rec.Body.String(), true)
	if got := dropper.ips; len(got) != 2 || got[0] != "203.0.113.1" || got[1] != "2001:db8::1" {
		t.Fatalf("dropper ips = %#v", got)
	}
}

func TestHandlerDropIPsRejectsInvalidRequests(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := HandlerController{state: state.NewRuntimeState(), logger: slog.Default(), dropper: &recordingConnectionDropper{}}

	assertSimpleSuccess(t, runHandlerRequest(t, ctrl.DropIPs, `{`).Body.String(), false)
	assertSimpleSuccess(t, runHandlerRequest(t, ctrl.DropIPs, `{"ips":[]}`).Body.String(), false)

	rec := runHandlerRequest(t, ctrl.DropIPs, `{"ips":["not-an-ip"]}`)
	assertSimpleSuccess(t, rec.Body.String(), false)
}

func TestHandlerDropIPsDegradesWhenConntrackUnavailable(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := HandlerController{
		state:   state.NewRuntimeState(),
		logger:  slog.Default(),
		dropper: &recordingConnectionDropper{err: fmt.Errorf("%w: fixture", system.ErrConntrackUnavailable)},
	}

	rec := runHandlerRequest(t, ctrl.DropIPs, `{"ips":["203.0.113.1"]}`)

	assertSimpleSuccess(t, rec.Body.String(), true)
}

func TestHandlerDropUsersConnectionsGetsUserIPs(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	stats := &recordingStatsClient{
		userIPs: []xray.IPLastSeen{{IP: "203.0.113.2", LastSeen: 1710000000}},
	}
	dropper := &recordingConnectionDropper{}
	ctrl := HandlerController{
		state:   state.NewRuntimeState(),
		logger:  slog.Default(),
		core:    &fakeCore{started: true, stats: stats},
		dropper: dropper,
	}

	rec := runHandlerRequest(t, ctrl.DropUsersConnections, `{"userIds":["user-1"]}`)

	assertSimpleSuccess(t, rec.Body.String(), true)
	if stats.onlineUsername != "" {
		t.Fatalf("online username should not be used: %q", stats.onlineUsername)
	}
	if !stats.userIPsReset {
		t.Fatalf("UserIPList reset = false, want true")
	}
	if got := dropper.ips; len(got) != 1 || got[0] != "203.0.113.2" {
		t.Fatalf("dropper ips = %#v", got)
	}
}

func TestHandlerDropUsersConnectionsDegradesWhenStatsUnavailable(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := HandlerController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{}}

	rec := runHandlerRequest(t, ctrl.DropUsersConnections, `{"userIds":["user-1"]}`)

	assertSimpleSuccess(t, rec.Body.String(), true)
}

func TestHandlerDropUsersConnectionsDegradesWhenUserIPLookupFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	stats := &recordingStatsClient{err: fmt.Errorf("boom")}
	dropper := &recordingConnectionDropper{}
	ctrl := HandlerController{
		state:   state.NewRuntimeState(),
		logger:  slog.Default(),
		core:    &fakeCore{started: true, stats: stats},
		dropper: dropper,
	}

	rec := runHandlerRequest(t, ctrl.DropUsersConnections, `{"userIds":["user-1"]}`)

	assertSimpleSuccess(t, rec.Body.String(), true)
	if len(dropper.ips) != 0 {
		t.Fatalf("dropper ips = %#v, want empty", dropper.ips)
	}
}

func runHandlerRequest(t *testing.T, handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/handler/test", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler(ctx)
	return rec
}

func assertGenericSuccess(t *testing.T, body string, want bool) {
	t.Helper()
	var decoded struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal generic response: %v; body=%s", err, body)
	}
	if decoded.Response.Success != want {
		t.Fatalf("success = %v, want %v; body=%s", decoded.Response.Success, want, body)
	}
}

func assertSimpleSuccess(t *testing.T, body string, want bool) {
	t.Helper()
	var decoded struct {
		Response struct {
			Success bool `json:"success"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal simple response: %v; body=%s", err, body)
	}
	if decoded.Response.Success != want {
		t.Fatalf("success = %v, want %v; body=%s", decoded.Response.Success, want, body)
	}
}

func assertOfficialErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string, wantMessage string, wantPath string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var body struct {
		Timestamp string `json:"timestamp"`
		Path      string `json:"path"`
		Message   string `json:"message"`
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error envelope: %v; body=%s", err, rec.Body.String())
	}
	if body.Timestamp == "" || body.Path != wantPath || body.Message != wantMessage || body.ErrorCode != wantCode {
		t.Fatalf("error body = %#v; raw=%s", body, rec.Body.String())
	}
}

type recordingHandlerClient struct {
	addSpecs    []xray.UserSpec
	removeCalls []struct {
		tag      string
		username string
	}
	users []xray.InboundUser
	count int
	err   error
}

func (c *recordingHandlerClient) AddUser(ctx context.Context, spec xray.UserSpec) error {
	c.addSpecs = append(c.addSpecs, spec)
	return c.err
}

func (c *recordingHandlerClient) RemoveUser(ctx context.Context, tag string, username string) error {
	c.removeCalls = append(c.removeCalls, struct {
		tag      string
		username string
	}{tag: tag, username: username})
	return c.err
}

func (c *recordingHandlerClient) GetInboundUsers(ctx context.Context, tag string) ([]xray.InboundUser, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.users, nil
}

func (c *recordingHandlerClient) GetInboundUsersCount(ctx context.Context, tag string) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	return c.count, nil
}

type recordingConnectionDropper struct {
	ips []string
	err error
}

func (d *recordingConnectionDropper) DropIP(ctx context.Context, ip string) error {
	d.ips = append(d.ips, ip)
	if d.err != nil {
		return d.err
	}
	if ip == "not-an-ip" {
		return fmt.Errorf("invalid ip %q", ip)
	}
	return nil
}
