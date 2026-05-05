package xray

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ProcessCore struct {
	binaryPath string
	configPath string
	apiAddress string
	startWait  time.Duration

	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan error
	version string
	grpc    *GRPCClient
	mtls    InternalMTLSBundle
}

func NewProcessCore(binaryPath, configPath, apiAddress string, mtls InternalMTLSBundle) *ProcessCore {
	grpcClient, err := NewGRPCClient(GRPCClientConfig{Address: apiAddress, MTLS: mtls})
	if err != nil {
		panic(err)
	}
	return &ProcessCore{
		binaryPath: binaryPath,
		configPath: configPath,
		apiAddress: apiAddress,
		startWait:  3 * time.Second,
		grpc:       grpcClient,
		mtls:       mtls,
	}
}

func (c *ProcessCore) Start(ctx context.Context, config []byte) error {
	if !json.Valid(config) {
		return fmt.Errorf("xray config is not valid JSON")
	}

	if err := os.MkdirAll(filepath.Dir(c.configPath), 0o755); err != nil {
		return fmt.Errorf("create xray config dir: %w", err)
	}
	if err := os.WriteFile(c.configPath, config, 0o600); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}

	if err := c.Stop(ctx); err != nil {
		return err
	}

	cmd := exec.Command(c.binaryPath, "run", "-config", c.configPath)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start xray: %w", err)
	}

	c.mu.Lock()
	c.cmd = cmd
	c.done = make(chan error, 1)
	done := c.done
	c.mu.Unlock()

	go func() {
		err := cmd.Wait()
		c.mu.Lock()
		if c.cmd == cmd {
			c.cmd = nil
			c.done = nil
		}
		c.mu.Unlock()
		done <- err
	}()

	if err := c.waitUntilReady(ctx); err != nil {
		_ = c.Stop(context.Background())
		tail := strings.TrimSpace(output.String())
		if tail != "" {
			return fmt.Errorf("%w: %s", err, truncate(tail, 512))
		}
		return err
	}

	version, err := c.Version(ctx)
	if err == nil && version != "" {
		c.mu.Lock()
		c.version = version
		c.mu.Unlock()
	}

	return nil
}

func (c *ProcessCore) Stop(ctx context.Context) error {
	c.mu.Lock()
	cmd := c.cmd
	done := c.done
	grpcClient := c.grpc
	c.mu.Unlock()
	if grpcClient != nil {
		_ = grpcClient.Close()
	}
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	_ = cmd.Process.Signal(os.Interrupt)

	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		if done != nil {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
	case <-done:
	}

	c.mu.Lock()
	if c.cmd == cmd {
		c.cmd = nil
	}
	c.mu.Unlock()
	return nil
}

func (c *ProcessCore) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cmd != nil && c.cmd.Process != nil
}

func (c *ProcessCore) Health(ctx context.Context) error {
	if !c.IsRunning() {
		return fmt.Errorf("xray is not running")
	}
	stats := c.Stats()
	if stats == nil {
		return fmt.Errorf("xray stats client is unavailable")
	}
	return stats.Ping(ctx)
}

func (c *ProcessCore) Version(ctx context.Context) (string, error) {
	c.mu.Lock()
	cached := c.version
	c.mu.Unlock()
	if cached != "" {
		return cached, nil
	}

	cmd := exec.CommandContext(ctx, c.binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get xray version: %w", err)
	}
	line := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	if line == "" {
		return "", nil
	}
	fields := strings.Fields(line)
	if len(fields) >= 2 && strings.EqualFold(fields[0], "Xray") {
		return fields[1], nil
	}
	return line, nil
}

func (c *ProcessCore) Handler() HandlerClient {
	c.mu.Lock()
	grpcClient := c.grpc
	c.mu.Unlock()
	if grpcClient == nil {
		return nil
	}
	client, err := grpcClient.Handler()
	if err != nil {
		return nil
	}
	return client
}

func (c *ProcessCore) Stats() StatsClient {
	c.mu.Lock()
	grpcClient := c.grpc
	c.mu.Unlock()
	if grpcClient == nil {
		return nil
	}
	client, err := grpcClient.Stats()
	if err != nil {
		return nil
	}
	return client
}

func (c *ProcessCore) Routing() RoutingClient {
	c.mu.Lock()
	grpcClient := c.grpc
	c.mu.Unlock()
	if grpcClient == nil {
		return nil
	}
	client, err := grpcClient.Routing()
	if err != nil {
		return nil
	}
	return client
}

func (c *ProcessCore) waitUntilReady(ctx context.Context) error {
	deadline := time.Now().Add(c.startWait)
	for {
		if !c.IsRunning() {
			return fmt.Errorf("xray exited before becoming ready")
		}
		healthCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err := c.Health(healthCtx)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("xray API did not become ready at %s", c.apiAddress)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
