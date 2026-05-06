package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/state"
)

func TestInternalGetConfigReturnsCurrentConfig(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	runtimeState.SetXrayStarted(nil, map[string]any{"stats": map[string]any{}}, state.Hashes{})
	ctrl := InternalController{state: runtimeState, logger: slog.Default()}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/internal/get-config", nil)
	ctrl.GetConfig(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v; body=%s", err, rec.Body.String())
	}
	if _, ok := body["stats"]; !ok {
		t.Fatalf("body = %#v", body)
	}
}
