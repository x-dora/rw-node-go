// Package geocheck execs the geocheck binary shipped in the official node
// image to produce a geocheck SVG report, mirroring the official GeocheckService.
package geocheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBinaryPath matches the official node image layout.
	DefaultBinaryPath = "/usr/local/bin/geocheck"
	// DefaultTimeout matches the official GEOCHECK_TIMEOUT_MS.
	DefaultTimeout = 45 * time.Second
	// DefaultMaxOutput matches the official GEOCHECK_MAX_OUTPUT.
	DefaultMaxOutput = 32 << 20
)

var (
	ErrAlreadyRunning = errors.New("geocheck: a run is already in progress")
	ErrNoImage        = errors.New("geocheck report carries no image")
)

type Request struct {
	IP        string
	Interface string
}

// Runner executes geocheck and returns the raw report JSON.
type Runner interface {
	Run(ctx context.Context, request Request) (json.RawMessage, error)
}

// Image enforces the only section the official contract guarantees; the rest
// of the report is passed through to the Panel unchanged (official looseObject).
type Image struct {
	Format    string `json:"format"`
	MediaType string `json:"media_type"`
	Encoding  string `json:"encoding"`
	Data      string `json:"data"`
}

type report struct {
	Image *Image `json:"image"`
}

// ExecRunner runs the geocheck binary shipped in the official node image.
type ExecRunner struct {
	// BinaryPath defaults to DefaultBinaryPath.
	BinaryPath string
	// Timeout defaults to DefaultTimeout.
	Timeout time.Duration
	// MaxOutput defaults to DefaultMaxOutput.
	MaxOutput int
	Logger    *slog.Logger

	// execCommand is an indirection for tests; production uses execOutput.
	execCommand func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error)

	mu        sync.Mutex
	isRunning bool
}

// NewRunner returns an ExecRunner with official defaults.
func NewRunner(logger *slog.Logger) *ExecRunner {
	return &ExecRunner{Logger: logger}
}

// Run executes geocheck for the request and returns the raw report JSON.
func (r *ExecRunner) Run(ctx context.Context, request Request) (json.RawMessage, error) {
	ip := strings.TrimSpace(request.IP)
	iface := strings.TrimSpace(request.Interface)

	var bindTo string
	if ip != "" {
		if net.ParseIP(ip) == nil {
			return nil, fmt.Errorf("geocheck: %q is not a valid IP address", ip)
		}
		bindTo = ip
	} else if iface != "" {
		bindTo = iface
	}

	if !r.begin() {
		return nil, ErrAlreadyRunning
	}
	defer r.end()

	target := bindTo
	if target == "" {
		target = "default route"
	}

	args := []string{"--json", "--svg-base64", "--quiet"}
	if bindTo != "" {
		args = append([]string{"--interface", bindTo}, args...)
	}

	binaryPath := r.BinaryPath
	if binaryPath == "" {
		binaryPath = DefaultBinaryPath
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxOutput := r.MaxOutput
	if maxOutput <= 0 {
		maxOutput = DefaultMaxOutput
	}

	execCommand := r.execCommand
	if execCommand == nil {
		execCommand = execOutput
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	stdout, err := execCommand(runCtx, binaryPath, args, maxOutput)
	if err != nil {
		return nil, formatExecError(runCtx, target, timeout, err)
	}

	var parsed report
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &parsed); err != nil {
		return nil, fmt.Errorf("geocheck via %s failed: %w", target, err)
	}
	if parsed.Image == nil || parsed.Image.Data == "" {
		return nil, ErrNoImage
	}

	if r.Logger != nil {
		r.Logger.Info("geocheck finished", "target", target, "elapsed", time.Since(started).Round(time.Millisecond))
	}
	return json.RawMessage(bytes.TrimSpace(stdout)), nil
}

func (r *ExecRunner) begin() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isRunning {
		return false
	}
	r.isRunning = true
	return true
}

func (r *ExecRunner) end() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.isRunning = false
}

func execOutput(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout limitedBuffer
	stdout.max = maxOutput
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if stdout.tooLarge {
			return nil, fmt.Errorf("geocheck output exceeded %d bytes", maxOutput)
		}
		return nil, err
	}
	return stdout.buf.Bytes(), nil
}

func formatExecError(ctx context.Context, target string, timeout time.Duration, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("geocheck via %s exceeded %s and was killed", target, timeout)
	}
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("geocheck via %s failed: binary not found", target)
	}
	return fmt.Errorf("geocheck via %s failed: %w", target, err)
}

type limitedBuffer struct {
	buf       bytes.Buffer
	max       int
	tooLarge  bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.max > 0 && b.buf.Len()+len(p) > b.max {
		b.tooLarge = true
		return 0, fmt.Errorf("output exceeds limit")
	}
	return b.buf.Write(p)
}

// ensure limitedBuffer satisfies io.Writer at compile time.
var _ io.Writer = (*limitedBuffer)(nil)
