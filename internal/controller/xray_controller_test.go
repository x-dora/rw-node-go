package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/version"
)

func TestHealthcheckStubIncludesNodeVersion(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	controller := XrayController{
		state:  runtimeState,
		logger: slog.Default(),
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

func TestStartXrayStubIsExplicitlyUnimplemented(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	controller := XrayController{
		state:  runtimeState,
		logger: slog.Default(),
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/node/xray/start", strings.NewReader(`{}`))

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
	if body.Response.Error == nil || *body.Response.Error != "not implemented" {
		t.Fatalf("Error = %v, want not implemented", body.Response.Error)
	}
}

func TestHandlerStubEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	httpapi.WriteHTTPEnvelope(rec, http.StatusOK, map[string]any{"success": true, "error": nil})

	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["response"]["success"] != true {
		t.Fatalf("success = %v, want true", body["response"]["success"])
	}
}
