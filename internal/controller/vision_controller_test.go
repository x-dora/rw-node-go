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
)

func TestVisionControllerUsesRoutingFeature(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	routing := &recordingRoutingClient{}
	ctrl := VisionController{
		state:  state.NewRuntimeState(),
		logger: slog.Default(),
		core:   &fakeCore{started: true, routing: routing},
	}

	rec := runVisionRequest(t, ctrl.BlockIP, `{"ip":"203.0.113.7","username":"user-1"}`)
	assertVisionSuccess(t, rec.Body.String(), true)
	if routing.addRuleTag != visionRuleTag("203.0.113.7") ||
		routing.addIP != "203.0.113.7" ||
		routing.addOutbound != xray.BlockOutboundTag {
		t.Fatalf("routing add = %#v", routing)
	}

	rec = runVisionRequest(t, ctrl.UnblockIP, `{"ip":"203.0.113.7","username":"user-1"}`)
	assertVisionSuccess(t, rec.Body.String(), true)
	if routing.removeRuleTag != visionRuleTag("203.0.113.7") {
		t.Fatalf("remove rule tag = %q", routing.removeRuleTag)
	}
}

func TestVisionControllerReturnsErrorWhenRoutingUnavailable(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := VisionController{
		state:  state.NewRuntimeState(),
		logger: slog.Default(),
		core:   &fakeCore{},
	}

	rec := runVisionRequest(t, ctrl.BlockIP, `{"ip":"203.0.113.7","username":"user-1"}`)
	assertVisionSuccess(t, rec.Body.String(), false)
}

func TestVisionControllerReturnsErrorForRoutingFailureAndInvalidJSON(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := VisionController{
		state:  state.NewRuntimeState(),
		logger: slog.Default(),
		core:   &fakeCore{started: true, routing: &recordingRoutingClient{err: errors.New("boom")}},
	}

	rec := runVisionRequest(t, ctrl.BlockIP, `{"ip":"203.0.113.7","username":"user-1"}`)
	assertVisionSuccess(t, rec.Body.String(), false)

	rec = runVisionRequest(t, ctrl.BlockIP, `{`)
	assertVisionSuccess(t, rec.Body.String(), false)
}

func runVisionRequest(t *testing.T, handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/vision/test", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	return rec
}

func assertVisionSuccess(t *testing.T, body string, want bool) {
	t.Helper()
	var decoded struct {
		Response struct {
			Success bool    `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal vision: %v; body=%s", err, body)
	}
	if decoded.Response.Success != want {
		t.Fatalf("success = %v, want %v; body=%s", decoded.Response.Success, want, body)
	}
	if want && decoded.Response.Error != nil {
		t.Fatalf("error = %q, want nil", *decoded.Response.Error)
	}
	if !want && decoded.Response.Error == nil {
		t.Fatalf("error = nil, want non-nil; body=%s", body)
	}
}

type recordingRoutingClient struct {
	addRuleTag    string
	addIP         string
	addOutbound   string
	removeRuleTag string
	err           error
}

func (c *recordingRoutingClient) AddSourceIPRule(ctx context.Context, ruleTag string, sourceIP string, outboundTag string) error {
	c.addRuleTag = ruleTag
	c.addIP = sourceIP
	c.addOutbound = outboundTag
	return c.err
}

func (c *recordingRoutingClient) RemoveRule(ctx context.Context, ruleTag string) error {
	c.removeRuleTag = ruleTag
	return c.err
}
