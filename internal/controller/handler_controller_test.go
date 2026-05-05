package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/xray"
	handlercommand "github.com/xtls/xray-core/app/proxyman/command"
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
		users: []xray.InboundUser{{Username: "user-1", Email: "user-1", Level: 0}},
		count: 7,
	}
	ctrl := HandlerController{state: state.NewRuntimeState(), logger: slog.Default(), core: &fakeCore{started: true, handler: handler}}

	usersRec := runHandlerRequest(t, ctrl.GetInboundUsers, `{"tag":"VLESS_INBOUND"}`)
	var usersBody struct {
		Response struct {
			Users []struct {
				Username string `json:"username"`
				Email    string `json:"email"`
				Level    int    `json:"level"`
			} `json:"users"`
		} `json:"response"`
	}
	if err := json.Unmarshal(usersRec.Body.Bytes(), &usersBody); err != nil {
		t.Fatalf("unmarshal users: %v", err)
	}
	if len(usersBody.Response.Users) != 1 || usersBody.Response.Users[0].Username != "user-1" {
		t.Fatalf("users body = %s", usersRec.Body.String())
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

func (c *recordingHandlerClient) Raw() handlercommand.HandlerServiceClient {
	return nil
}
