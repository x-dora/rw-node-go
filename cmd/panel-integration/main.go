package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/x-dora/rw-node-go/internal/testkit"
)

const (
	scriptGateEnv        = "RW_PANEL_INTEGRATION_SCRIPT"
	panelSmokePathEnv    = "PANEL_SMOKE_PATH"
	extendedSmokeEnv     = "PANEL_EXTENDED_SMOKE"
	defaultPanelSmokeAPI = "/api/system/metadata"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage(stdout)
		if len(args) == 0 {
			return 2
		}
		return 0
	}

	command := args[0]
	if strings.TrimSpace(os.Getenv(scriptGateEnv)) != "1" {
		fmt.Fprintf(stderr, "%s=1 is required; use bash scripts/panel-integration.sh %s\n", scriptGateEnv, command)
		return 2
	}

	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	enableTimeout := fs.Duration("enable-timeout", 75*time.Second, "timeout while waiting for Panel to report connected")
	disableTimeout := fs.Duration("disable-timeout", 45*time.Second, "timeout while waiting for Panel to report disabled")
	pollInterval := fs.Duration("poll-interval", 5*time.Second, "Panel node status polling interval")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	logger := slog.New(slog.NewJSONHandler(stdout, nil))
	client, err := testkit.NewPanelClientFromEnv(logger)
	if err != nil {
		logEvent(logger, "error", "panel_config", err.Error())
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout(command, *enableTimeout, *disableTimeout))
	defer cancel()

	switch command {
	case "smoke":
		err = runSmoke(ctx, logger, client)
	case "extended-smoke":
		err = runExtendedSmoke(ctx, logger, client)
	case "node":
		err = runNode(ctx, logger, client)
	case "enable":
		err = runEnable(ctx, logger, client, *enableTimeout, *pollInterval)
	case "disable":
		err = runDisable(ctx, logger, client, *disableTimeout, *pollInterval)
	default:
		printUsage(stderr)
		logEvent(logger, "error", "unknown_command", command)
		return 2
	}
	if err != nil {
		logEvent(logger, "error", "panel_integration_failed", err.Error())
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: panel-integration <command>

Internal script-only Remnawave Panel live harness.
Use bash scripts/panel-integration.sh <command> instead of running this command directly.

Commands:
  smoke           Run the live Panel smoke request.
  extended-smoke  Run optional read-only Panel smoke requests configured by environment.
  node            Query Panel for the configured node status summary.
  enable          Enable the configured Panel node and wait for isConnected=true.
  disable         Disable the configured Panel node and wait for isDisabled=true.`)
}

func commandTimeout(command string, enableTimeout time.Duration, disableTimeout time.Duration) time.Duration {
	switch command {
	case "enable":
		return enableTimeout + 30*time.Second
	case "disable":
		return disableTimeout + 30*time.Second
	case "extended-smoke":
		return 2 * time.Minute
	default:
		return 30 * time.Second
	}
}

func runSmoke(ctx context.Context, logger *slog.Logger, client *testkit.PanelClient) error {
	summary, err := json.Marshal(client.ConfigSummary())
	if err != nil {
		return fmt.Errorf("marshal panel config summary: %w", err)
	}
	logEvent(logger, "info", "panel_config", string(summary))

	path := strings.TrimSpace(os.Getenv(panelSmokePathEnv))
	if path == "" {
		path = defaultPanelSmokeAPI
	}

	resp, err := client.Get(ctx, path)
	if err != nil {
		logPanelResponse(logger, "panel_smoke_response", resp)
		return fmt.Errorf("panel smoke request failed: %w", err)
	}
	logger.Info("panel smoke",
		"event", "panel_smoke",
		"status", resp.StatusCode,
		"category", resp.ErrorCategory,
		"duration_ms", resp.Duration.Milliseconds(),
	)
	if resp.PrettyBody != "" {
		logger.Info("panel smoke body", "event", "panel_smoke_body", "body", resp.PrettyBody)
	}
	return nil
}

func runExtendedSmoke(ctx context.Context, logger *slog.Logger, client *testkit.PanelClient) error {
	rawPaths := strings.TrimSpace(os.Getenv(extendedSmokeEnv))
	if rawPaths == "" {
		logEvent(logger, "info", "panel_extended_smoke_skipped", extendedSmokeEnv+" is empty")
		return nil
	}

	paths := splitSmokePaths(rawPaths)
	if len(paths) == 0 {
		logEvent(logger, "info", "panel_extended_smoke_skipped", extendedSmokeEnv+" has no usable paths")
		return nil
	}

	for _, path := range paths {
		resp, err := client.Get(ctx, path)
		if err != nil {
			logPanelResponse(logger, "panel_extended_smoke_response", resp)
			return fmt.Errorf("panel extended smoke request %s failed: %w", path, err)
		}
		logger.Info("panel extended smoke",
			"event", "panel_extended_smoke",
			"path", path,
			"status", resp.StatusCode,
			"category", resp.ErrorCategory,
			"duration_ms", resp.Duration.Milliseconds(),
		)
	}
	return nil
}

func runNode(ctx context.Context, logger *slog.Logger, client *testkit.PanelClient) error {
	nodeHint := strings.TrimSpace(os.Getenv(testkit.PanelNodeIDEnv))
	if nodeHint == "" {
		return fmt.Errorf("%s is required for node status lookup", testkit.PanelNodeIDEnv)
	}

	summary, resp, err := client.GetNodeStatusSummaryByHint(ctx, nodeHint)
	if err != nil {
		logPanelResponse(logger, "panel_node_response", resp)
		return fmt.Errorf("panel node status lookup failed: %w", err)
	}
	logger.Info("panel node status",
		"event", "panel_node_status",
		"status", resp.StatusCode,
		"category", resp.ErrorCategory,
		"duration_ms", resp.Duration.Milliseconds(),
	)
	logNodeSummary(logger, "panel_node", summary)
	return nil
}

func splitSmokePaths(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		path := strings.TrimSpace(field)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func runEnable(ctx context.Context, logger *slog.Logger, client *testkit.PanelClient, timeout time.Duration, pollInterval time.Duration) error {
	nodeUUID := strings.TrimSpace(os.Getenv(testkit.PanelNodeIDEnv))
	if nodeUUID == "" {
		return fmt.Errorf("%s is required for node enable/connect lookup", testkit.PanelNodeIDEnv)
	}
	if !testkit.IsUUID(nodeUUID) {
		return fmt.Errorf("%s must be a full node UUID for enable/disable commands because they modify real Panel node state", testkit.PanelNodeIDEnv)
	}
	if pollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}

	summary, _, err := client.GetNodeStatusSummaryByUUID(ctx, nodeUUID)
	if err != nil {
		return fmt.Errorf("resolve panel node: %w", err)
	}

	enabled, resp, err := client.EnableNode(ctx, summary.UUID)
	if err != nil {
		logPanelResponse(logger, "panel_node_enable_response", resp)
		return fmt.Errorf("enable panel node %s: %w", summary.UUID, err)
	}
	logNodeSummary(logger, "panel_node_enabled", enabled)

	deadline := time.Now().Add(timeout)
	current, err := waitForNode(ctx, client, summary.UUID, pollInterval, deadline, func(current *testkit.NodeStatusSummary) bool {
		logNodeSummary(logger, "panel_node_poll", current)
		return current.IsConnected && !current.IsDisabled
	})
	if err != nil {
		if current != nil {
			return fmt.Errorf("panel node did not become connected: uuid=%s name=%s connected=%t connecting=%t disabled=%t last_status_message=%v: %w",
				current.UUID,
				current.Name,
				current.IsConnected,
				current.IsConnecting,
				current.IsDisabled,
				current.LastStatusMessage,
				err,
			)
		}
		return err
	}
	logNodeSummary(logger, "panel_node_connected", current)
	return nil
}

func runDisable(ctx context.Context, logger *slog.Logger, client *testkit.PanelClient, timeout time.Duration, pollInterval time.Duration) error {
	nodeUUID := strings.TrimSpace(os.Getenv(testkit.PanelNodeIDEnv))
	if nodeUUID == "" {
		return fmt.Errorf("%s is required for node disable lookup", testkit.PanelNodeIDEnv)
	}
	if !testkit.IsUUID(nodeUUID) {
		return fmt.Errorf("%s must be a full node UUID for enable/disable commands because they modify real Panel node state", testkit.PanelNodeIDEnv)
	}
	if pollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}

	summary, _, err := client.GetNodeStatusSummaryByUUID(ctx, nodeUUID)
	if err != nil {
		return fmt.Errorf("resolve panel node: %w", err)
	}

	disabled, resp, err := client.DisableNode(ctx, summary.UUID)
	if err != nil {
		logPanelResponse(logger, "panel_node_disable_response", resp)
		return fmt.Errorf("disable panel node %s: %w", summary.UUID, err)
	}
	logNodeSummary(logger, "panel_node_disabled_response", disabled)

	deadline := time.Now().Add(timeout)
	current, err := waitForNode(ctx, client, summary.UUID, pollInterval, deadline, func(current *testkit.NodeStatusSummary) bool {
		logNodeSummary(logger, "panel_node_disable_poll", current)
		return current.IsDisabled
	})
	if err != nil {
		if current != nil {
			return fmt.Errorf("panel node did not become disabled: uuid=%s name=%s connected=%t connecting=%t disabled=%t last_status_message=%v: %w",
				current.UUID,
				current.Name,
				current.IsConnected,
				current.IsConnecting,
				current.IsDisabled,
				current.LastStatusMessage,
				err,
			)
		}
		return err
	}
	logNodeSummary(logger, "panel_node_disabled", current)
	return nil
}

func waitForNode(
	ctx context.Context,
	client *testkit.PanelClient,
	nodeUUID string,
	pollInterval time.Duration,
	deadline time.Time,
	matches func(*testkit.NodeStatusSummary) bool,
) (*testkit.NodeStatusSummary, error) {
	var last *testkit.NodeStatusSummary
	for {
		current, _, err := client.GetNodeStatusSummaryByUUID(ctx, nodeUUID)
		if err != nil {
			return last, fmt.Errorf("poll panel node %s: %w", nodeUUID, err)
		}
		last = current
		if matches(current) {
			return current, nil
		}
		if time.Now().After(deadline) {
			return last, errors.New("timed out waiting for panel node status")
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func logNodeSummary(logger *slog.Logger, event string, summary *testkit.NodeStatusSummary) {
	if summary == nil {
		return
	}
	logger.Info("panel node",
		"event", event,
		"uuid", summary.UUID,
		"name", summary.Name,
		"address", summary.Address,
		"port", summary.Port,
		"connected", summary.IsConnected,
		"connecting", summary.IsConnecting,
		"disabled", summary.IsDisabled,
		"last_status_change", summary.LastStatusChange,
		"last_status_message", redactAny(summary.LastStatusMessage),
		"country", summary.CountryCode,
	)
}

func logPanelResponse(logger *slog.Logger, event string, resp *testkit.PanelResponse) {
	if resp == nil {
		return
	}
	logger.Info("panel response",
		"event", event,
		"status", resp.StatusCode,
		"category", resp.ErrorCategory,
		"duration_ms", resp.Duration.Milliseconds(),
		"body", testkit.RedactText(resp.PrettyBody),
	)
}

func logEvent(logger *slog.Logger, level string, event string, message string) {
	message = testkit.RedactText(message)
	switch level {
	case "error":
		logger.Error(message, "event", event)
	case "warn":
		logger.Warn(message, "event", event)
	default:
		logger.Info(message, "event", event)
	}
}

func redactAny(value any) any {
	if text, ok := value.(string); ok {
		return testkit.RedactText(text)
	}
	return value
}
