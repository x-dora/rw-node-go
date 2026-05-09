package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/x-dora/rw-node-go/internal/testkit"
)

func TestMutatingCommandsRequireFullUUID(t *testing.T) {
	t.Setenv(scriptGateEnv, "1")
	t.Setenv("PANEL_BASE_URL", "https://panel.example")
	t.Setenv("PANEL_API_KEY", "test-api-key")
	t.Setenv("PANEL_NODE_ID", "node-name")

	var stdout strings.Builder
	var stderr strings.Builder
	code := run([]string{"enable"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(enable) code = %d, want 1\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	got := stdout.String() + stderr.String()
	if !strings.Contains(got, "must be a full node UUID") {
		t.Fatalf("run(enable) output missing UUID guidance:\n%s", got)
	}
}

func TestEnableWithFullUUIDCallsActionEndpoint(t *testing.T) {
	nodeUUID := "11111111-1111-1111-1111-111111111111"
	var actionPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/nodes/"+nodeUUID:
			_ = json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{
				"uuid":        nodeUUID,
				"name":        "test-node",
				"isConnected": true,
				"isDisabled":  false,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/nodes/"+nodeUUID+"/actions/enable":
			actionPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{
				"uuid":        nodeUUID,
				"name":        "test-node",
				"isConnected": false,
				"isDisabled":  false,
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv(scriptGateEnv, "1")
	t.Setenv("PANEL_BASE_URL", server.URL)
	t.Setenv("PANEL_API_KEY", "test-api-key")
	t.Setenv("PANEL_NODE_ID", nodeUUID)

	var stdout strings.Builder
	var stderr strings.Builder
	code := run([]string{"enable", "-enable-timeout", "1s", "-poll-interval", "1ms"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(enable) code = %d, want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if actionPath != "/api/nodes/"+nodeUUID+"/actions/enable" {
		t.Fatalf("actionPath = %q, want enable endpoint", actionPath)
	}
}

func TestPanelLogsRedactFreeFormText(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	jwt := "aaaaaaaaaa.bbbbbbbbbb.cccccccccc"

	logNodeSummary(logger, "panel_node_poll", &testkit.NodeStatusSummary{
		UUID:              "11111111-1111-1111-1111-111111111111",
		LastStatusMessage: "Authorization: Bearer " + jwt,
	})
	logPanelResponse(logger, "panel_response", &testkit.PanelResponse{
		StatusCode:    500,
		ErrorCategory: "panel_server_error",
		Duration:      time.Millisecond,
		PrettyBody:    "secretKey=very-secret-value",
	})
	logEvent(logger, "error", "panel_error", "token=message-secret")

	got := logBuffer.String()
	for _, leaked := range []string{jwt, "very-secret-value", "message-secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("logs leaked %q:\n%s", leaked, got)
		}
	}
}

func TestExtendedSmokeUsesConfiguredPaths(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"ok": true}})
	}))
	defer server.Close()

	t.Setenv(scriptGateEnv, "1")
	t.Setenv("PANEL_BASE_URL", server.URL)
	t.Setenv("PANEL_API_KEY", "test-api-key")
	t.Setenv("PANEL_EXTENDED_SMOKE", "/api/system/metadata,/api/nodes")

	var stdout strings.Builder
	var stderr strings.Builder
	code := run([]string{"extended-smoke"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(extended-smoke) code = %d, want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Join(paths, ",") != "/api/system/metadata,/api/nodes" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
