package geocheck

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunnerReturnsRawReport(t *testing.T) {
	report := `{"image":{"format":"svg","media_type":"image/svg+xml","encoding":"base64","data":"abc"},"ip":"203.0.113.1"}`
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		if path != DefaultBinaryPath {
			t.Fatalf("path = %q, want %q", path, DefaultBinaryPath)
		}
		if len(args) != 5 || args[0] != "--interface" || args[1] != "203.0.113.1" || args[2] != "--json" || args[3] != "--svg-base64" || args[4] != "--quiet" {
			t.Fatalf("args = %#v", args)
		}
		return []byte(report + "\n"), nil
	}}

	got, err := runner.Run(context.Background(), Request{IP: "203.0.113.1"})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if strings.TrimSpace(string(got)) != report {
		t.Fatalf("report = %s", got)
	}
}

func TestRunnerUsesEnvBinaryPath(t *testing.T) {
	t.Setenv("GEOCHECK_BINARY_PATH", "/opt/rw-node/bin/geocheck")
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		if path != "/opt/rw-node/bin/geocheck" {
			t.Fatalf("path = %q, want /opt/rw-node/bin/geocheck", path)
		}
		return []byte(`{"image":{"data":"abc"}}`), nil
	}}

	if _, err := runner.Run(context.Background(), Request{}); err != nil {
		t.Fatalf("Run err = %v", err)
	}
}

func TestRunnerEnvBinaryPathBlankFallsBackToDefault(t *testing.T) {
	t.Setenv("GEOCHECK_BINARY_PATH", "   ")
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		if path != DefaultBinaryPath {
			t.Fatalf("path = %q, want %q", path, DefaultBinaryPath)
		}
		return []byte(`{"image":{"data":"abc"}}`), nil
	}}

	if _, err := runner.Run(context.Background(), Request{}); err != nil {
		t.Fatalf("Run err = %v", err)
	}
}

func TestRunnerFieldBinaryPathOverridesEnv(t *testing.T) {
	t.Setenv("GEOCHECK_BINARY_PATH", "/opt/rw-node/bin/geocheck")
	runner := &ExecRunner{BinaryPath: "/custom/geocheck", execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		if path != "/custom/geocheck" {
			t.Fatalf("path = %q, want /custom/geocheck", path)
		}
		return []byte(`{"image":{"data":"abc"}}`), nil
	}}

	if _, err := runner.Run(context.Background(), Request{}); err != nil {
		t.Fatalf("Run err = %v", err)
	}
}

func TestRunnerPassesInterfaceWhenIPMissing(t *testing.T) {
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		if len(args) != 5 || args[0] != "--interface" || args[1] != "eth0" {
			t.Fatalf("args = %#v", args)
		}
		return []byte(`{"image":{"data":"abc"}}`), nil
	}}

	if _, err := runner.Run(context.Background(), Request{Interface: "eth0"}); err != nil {
		t.Fatalf("Run err = %v", err)
	}
}

func TestRunnerRunsDefaultRouteWithoutBind(t *testing.T) {
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		if len(args) != 3 {
			t.Fatalf("args = %#v", args)
		}
		return []byte(`{"image":{"data":"abc"}}`), nil
	}}

	if _, err := runner.Run(context.Background(), Request{}); err != nil {
		t.Fatalf("Run err = %v", err)
	}
}

func TestRunnerRejectsInvalidIP(t *testing.T) {
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		t.Fatalf("exec should not run for invalid ip")
		return nil, nil
	}}

	_, err := runner.Run(context.Background(), Request{IP: "not-an-ip"})
	if err == nil || !strings.Contains(err.Error(), "not a valid IP address") {
		t.Fatalf("err = %v, want invalid ip message", err)
	}
}

func TestRunnerRejectsReportWithoutImage(t *testing.T) {
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		return []byte(`{"ip":"203.0.113.1"}`), nil
	}}

	if _, err := runner.Run(context.Background(), Request{}); !errors.Is(err, ErrNoImage) {
		t.Fatalf("err = %v, want ErrNoImage", err)
	}
}

func TestRunnerMapsExecError(t *testing.T) {
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		return nil, errors.New("boom")
	}}

	_, err := runner.Run(context.Background(), Request{IP: "203.0.113.1"})
	if err == nil || !strings.Contains(err.Error(), "geocheck via 203.0.113.1 failed: boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunnerMapsContextDeadlineToTimeoutMessage(t *testing.T) {
	runner := &ExecRunner{Timeout: 20 * time.Millisecond, execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}

	_, err := runner.Run(context.Background(), Request{})
	if err == nil || !strings.Contains(err.Error(), "exceeded 20ms and was killed") {
		t.Fatalf("err = %v, want timeout message", err)
	}
}

func TestRunnerRejectsConcurrentRuns(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	runner := &ExecRunner{execCommand: func(ctx context.Context, path string, args []string, maxOutput int) ([]byte, error) {
		once.Do(func() { close(started) })
		<-release
		return []byte(`{"image":{"data":"abc"}}`), nil
	}}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := runner.Run(context.Background(), Request{}); err != nil {
			t.Errorf("first run err = %v", err)
		}
	}()

	<-started
	if _, err := runner.Run(context.Background(), Request{}); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second run err = %v, want ErrAlreadyRunning", err)
	}
	close(release)
	wg.Wait()

	if _, err := runner.Run(context.Background(), Request{}); err != nil {
		t.Fatalf("run after release err = %v", err)
	}
}
