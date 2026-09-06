package controller

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/testkit"
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
	if _, ok := body["panelSni"]; ok {
		t.Fatalf("panelSni must be absent without a derived SNI, body = %#v", body)
	}
}

func TestInternalGetConfigInjectsPanelSni(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()
	runtimeState.SetXrayStarted(nil, map[string]any{"stats": map[string]any{}}, state.Hashes{})

	bundle := testkit.NewCertBundle(t)
	secretKey := encodePayloadSecret(t, bundle.Payload)
	sni := derivePanelSniForTest(t, secretKey)

	ctrl := InternalController{state: runtimeState, logger: slog.Default(), panelSni: sni}

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
	if got := body["panelSni"]; got != sni {
		t.Fatalf("panelSni = %v, want %s; body = %#v", got, sni, body)
	}
	if _, ok := body["stats"]; !ok {
		t.Fatalf("stats must survive the panelSni injection, body = %#v", body)
	}
}

func TestInternalGetConfigInjectsPanelSniOnEmptyState(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runtimeState := state.NewRuntimeState()

	bundle := testkit.NewCertBundle(t)
	secretKey := encodePayloadSecret(t, bundle.Payload)
	sni := derivePanelSniForTest(t, secretKey)

	ctrl := InternalController{state: runtimeState, logger: slog.Default(), panelSni: sni}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/internal/get-config", nil)
	ctrl.GetConfig(ctx)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v; body=%s", err, rec.Body.String())
	}
	if got := body["panelSni"]; got != sni {
		t.Fatalf("panelSni = %v, want %s; body = %#v", got, sni, body)
	}
}

func encodePayloadSecret(t *testing.T, payload any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func derivePanelSniForTest(t *testing.T, secretKey string) string {
	t.Helper()
	t.Setenv("SECRET_KEY", secretKey)
	sni := derivePanelSni(slog.Default())
	if sni == "" {
		t.Fatalf("derivePanelSni returned empty for a valid SECRET_KEY")
	}
	return sni
}
