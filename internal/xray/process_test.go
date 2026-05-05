package xray

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestProcessCoreVersion(t *testing.T) {
	fake := fakeXray(t, `if "%1"=="version" (echo Xray 25.1.1 ^(test^)& exit /b 0)
exit /b 1
`)
	if runtime.GOOS != "windows" {
		fake = fakeXray(t, `if [ "$1" = "version" ]; then echo "Xray 25.1.1 (test)"; exit 0; fi
exit 1
`)
	}

	core := NewProcessCore(fake, filepath.Join(t.TempDir(), "config.json"), "127.0.0.1:1")
	version, err := core.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version != "25.1.1" {
		t.Fatalf("version = %q, want 25.1.1", version)
	}
}

func TestProcessCoreStartWritesConfigAndFailsWhenAPIUnavailable(t *testing.T) {
	fake := fakeXray(t, `if "%1"=="run" (
  ping -n 6 127.0.0.1 >NUL
  exit /b 0
)
if "%1"=="version" (echo Xray 25.1.1& exit /b 0)
exit /b 1
`)
	if runtime.GOOS != "windows" {
		fake = fakeXray(t, `if [ "$1" = "run" ]; then sleep 5; exit 0; fi
if [ "$1" = "version" ]; then echo "Xray 25.1.1"; exit 0; fi
exit 1
`)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "xray", "config.json")
	core := NewProcessCore(fake, configPath, "127.0.0.1:1")
	core.startWait = 100 * time.Millisecond
	err := core.Start(context.Background(), []byte(`{"log":{}}`))
	if err == nil {
		t.Fatalf("Start() error = nil, want API unavailable")
	}
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if string(data) != `{"log":{}}` {
		t.Fatalf("config = %q", data)
	}
}

func fakeXray(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "xray.bat")
		if err := os.WriteFile(path, []byte("@echo off\r\n"+script), 0o755); err != nil {
			t.Fatalf("write fake xray: %v", err)
		}
		return path
	}

	path := filepath.Join(dir, "xray")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("write fake xray: %v", err)
	}
	return path
}
