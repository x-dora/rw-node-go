package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/x-dora/rw-node-go/internal/testkit"
)

func TestPanelIntegrationScriptHelp(t *testing.T) {
	requireBash(t)
	root := testkit.ProjectRoot(t)
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "panel-integration.sh"), "help")
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("help command error = %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "Usage: scripts/panel-integration.sh") {
		t.Fatalf("help output missing usage:\n%s", output)
	}
}

func TestPanelIntegrationScriptRunRequiresEnv(t *testing.T) {
	requireBash(t)
	root := testkit.ProjectRoot(t)
	missingEnvFile := filepath.Join(t.TempDir(), "missing.env")
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "panel-integration.sh"), "run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PANEL_INTEGRATION_ENV_FILE="+missingEnvFile)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("run command succeeded without env; output:\n%s", output)
	}
	if !strings.Contains(string(output), "Missing required env") {
		t.Fatalf("run output missing actionable error:\n%s", output)
	}
}

func TestPanelIntegrationScriptRunRequiresFullNodeUUID(t *testing.T) {
	requireBash(t)
	root := testkit.ProjectRoot(t)
	envFile := filepath.Join(t.TempDir(), "integration.env")
	content := strings.Join([]string{
		"PANEL_BASE_URL=https://panel.example",
		"PANEL_API_KEY=test-api-key",
		"PANEL_NODE_ID=node-name",
		"SECRET_KEY=test-secret",
		"",
	}, "\n")
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cmd := exec.Command("bash", filepath.Join(root, "scripts", "panel-integration.sh"), "run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PANEL_INTEGRATION_ENV_FILE="+envFile)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("run command succeeded with fuzzy node id; output:\n%s", output)
	}
	if !strings.Contains(string(output), "PANEL_NODE_ID must be a full node UUID") {
		t.Fatalf("run output missing UUID guidance:\n%s", output)
	}
}

func TestPanelIntegrationScriptStatus(t *testing.T) {
	requireBash(t)
	root := testkit.ProjectRoot(t)
	envFile := filepath.Join(t.TempDir(), "integration.env")
	if err := os.WriteFile(envFile, []byte("NODE_PORT=2222\nINTERNAL_REST_PORT=61001\nPANEL_INTEGRATION_LOG_DIR=logs/test-panel-integration\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cmd := exec.Command("bash", filepath.Join(root, "scripts", "panel-integration.sh"), "status")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PANEL_INTEGRATION_ENV_FILE="+envFile)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status command error = %v\n%s", err, output)
	}
	got := string(output)
	if !strings.Contains(got, `"node_port": "2222"`) || !strings.Contains(got, `"event":"node_status"`) {
		t.Fatalf("status output missing summary/status:\n%s", got)
	}
}

func TestPanelIntegrationScriptStopRefusesLegacyPIDFile(t *testing.T) {
	requireBash(t)
	root := testkit.ProjectRoot(t)
	tempDir := t.TempDir()
	logDir := filepath.ToSlash(filepath.Join(tempDir, "logs"))
	envFile := filepath.Join(tempDir, "integration.env")
	content := strings.Join([]string{
		"NODE_PORT=2222",
		"INTERNAL_REST_PORT=61001",
		"PANEL_INTEGRATION_LOG_DIR=" + logDir,
		"",
	}, "\n")
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "rw-node-go.pid.json"), []byte("12345"), 0o600); err != nil {
		t.Fatalf("write legacy pid file: %v", err)
	}

	cmd := exec.Command("bash", filepath.Join(root, "scripts", "panel-integration.sh"), "stop")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PANEL_INTEGRATION_ENV_FILE="+envFile)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("stop command succeeded with legacy pid file; output:\n%s", output)
	}
	if !strings.Contains(string(output), "legacy pid file") || !strings.Contains(string(output), "refusing") {
		t.Fatalf("stop output missing stale pid warning:\n%s", output)
	}
}

func TestPanelIntegrationScriptRunReportsDisableCleanupFailure(t *testing.T) {
	requireBash(t)
	root := testkit.ProjectRoot(t)
	tempDir := t.TempDir()
	fakeBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	fakeGo := filepath.Join(fakeBin, "go")
	if err := os.WriteFile(fakeGo, []byte(`#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "env" && "${2:-}" == "GOEXE" ]]; then
  exit 0
fi
if [[ "${1:-}" == "build" ]]; then
  out=""
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "-o" ]]; then
      shift
      out="$1"
      break
    fi
    shift
  done
  cat >"$out" <<'NODE'
#!/usr/bin/env bash
trap 'exit 0' TERM INT
while true; do sleep 1; done
NODE
  chmod +x "$out"
  exit 0
fi
if [[ "${1:-}" == "run" && "${2:-}" == "./cmd/panel-integration" ]]; then
  cmd="${3:-}"
  case "$cmd" in
    enable) exit 0 ;;
    smoke) exit 0 ;;
    disable)
      echo '{"level":"error","event":"panel_node_disable_cleanup_failed","message":"simulated disable failure"}'
      exit 1
      ;;
  esac
fi
echo "unexpected fake go args: $*" >&2
exit 1
`), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}
	fakeMise := filepath.Join(fakeBin, "mise")
	if err := os.WriteFile(fakeMise, []byte(`#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "exec" && "${2:-}" == "--" && "${3:-}" == "go" ]]; then
  shift 3
  exec go "$@"
fi
echo "unexpected fake mise args: $*" >&2
exit 1
`), 0o755); err != nil {
		t.Fatalf("write fake mise: %v", err)
	}

	envFile := filepath.Join(tempDir, "integration.env")
	content := strings.Join([]string{
		"PANEL_BASE_URL=https://panel.example",
		"PANEL_API_KEY=test-api-key",
		"PANEL_NODE_ID=11111111-1111-1111-1111-111111111111",
		"SECRET_KEY=test-secret",
		"PANEL_INTEGRATION_LOG_DIR=" + filepath.ToSlash(filepath.Join(tempDir, "logs")),
		"PANEL_INTEGRATION_BIN_DIR=" + filepath.ToSlash(filepath.Join(tempDir, "runtime", "bin")),
		"XRAY_ASSET_DIR=" + filepath.ToSlash(filepath.Join(tempDir, "runtime", "xray")),
		"",
	}, "\n")
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cmd := exec.Command("bash", filepath.Join(root, "scripts", "panel-integration.sh"), "run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PANEL_INTEGRATION_ENV_FILE="+envFile, "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("run command succeeded when disable failed; output:\n%s", output)
	}
	if !strings.Contains(string(output), "panel_node_disable_cleanup_failed") {
		t.Fatalf("run output missing disable cleanup failure:\n%s", output)
	}
}

func TestPanelIntegrationHarnessRequiresScriptGate(t *testing.T) {
	root := testkit.ProjectRoot(t)
	cmd := exec.Command("go", "run", "./cmd/panel-integration", "smoke")
	cmd.Dir = root
	cmd.Env = withoutEnv(os.Environ(), "RW_PANEL_INTEGRATION_SCRIPT")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("harness succeeded without script gate; output:\n%s", output)
	}
	got := string(output)
	if !strings.Contains(got, "RW_PANEL_INTEGRATION_SCRIPT=1 is required") ||
		!strings.Contains(got, "bash scripts/panel-integration.sh smoke") {
		t.Fatalf("harness output missing script-only guidance:\n%s", got)
	}
}

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash script test requires bash")
	}
}

func withoutEnv(env []string, name string) []string {
	prefix := name + "="
	out := make([]string, 0, len(env))
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		out = append(out, item)
	}
	return out
}
