package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	chdir(t, t.TempDir())
	clearEnv(t,
		"NODE_PORT",
		"INTERNAL_REST_PORT",
		"SECRET_KEY",
		"RW_NODE_DIR",
		"LOG_LEVEL",
		"REQUEST_BODY_LIMIT_BYTES",
		"REQUIRE_SECRET_KEY",
		"ALLOW_INSECURE_HTTP_TARGET",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.NodePort != DefaultNodePort {
		t.Fatalf("NodePort = %d, want %d", cfg.NodePort, DefaultNodePort)
	}
	if cfg.InternalRESTPort != DefaultInternalRESTPort {
		t.Fatalf("InternalRESTPort = %d, want %d", cfg.InternalRESTPort, DefaultInternalRESTPort)
	}
	if cfg.RWNodeDir != DefaultRWNodeDir {
		t.Fatalf("RWNodeDir = %q, want %q", cfg.RWNodeDir, DefaultRWNodeDir)
	}
}

func TestLoadReadsDotEnv(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearEnv(t,
		"NODE_PORT",
		"INTERNAL_REST_PORT",
		"SECRET_KEY",
		"RW_NODE_DIR",
		"LOG_LEVEL",
		"REQUEST_BODY_LIMIT_BYTES",
		"REQUIRE_SECRET_KEY",
		"ALLOW_INSECURE_HTTP_TARGET",
	)
	writeFile(t, filepath.Join(dir, ".env"), strings.Join([]string{
		"NODE_PORT=3333",
		"INTERNAL_REST_PORT=62000",
		"LOG_LEVEL=debug",
		"SECRET_KEY=dotenv-secret",
		"",
	}, "\n"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.NodePort != 3333 {
		t.Fatalf("NodePort = %d, want 3333", cfg.NodePort)
	}
	if cfg.InternalRESTPort != 62000 {
		t.Fatalf("InternalRESTPort = %d, want 62000", cfg.InternalRESTPort)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", cfg.LogLevel)
	}
	if cfg.SecretKey != "dotenv-secret" {
		t.Fatalf("SecretKey was not loaded from .env")
	}
}

func TestLoadEnvOverridesDotEnv(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearEnv(t, "INTERNAL_REST_PORT", "SECRET_KEY", "RW_NODE_DIR", "LOG_LEVEL", "REQUEST_BODY_LIMIT_BYTES", "REQUIRE_SECRET_KEY", "ALLOW_INSECURE_HTTP_TARGET")
	t.Setenv("NODE_PORT", "4444")
	writeFile(t, filepath.Join(dir, ".env"), "NODE_PORT=3333\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.NodePort != 4444 {
		t.Fatalf("NodePort = %d, want 4444", cfg.NodePort)
	}
}

func TestLoadRejectsMalformedDotEnv(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearEnv(t, "NODE_PORT", "INTERNAL_REST_PORT", "SECRET_KEY", "RW_NODE_DIR", "LOG_LEVEL", "REQUEST_BODY_LIMIT_BYTES", "REQUIRE_SECRET_KEY", "ALLOW_INSECURE_HTTP_TARGET")
	writeFile(t, filepath.Join(dir, ".env"), "NODE_PORT=\"unterminated\n")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load() error = nil, want malformed .env error")
	}
	if !strings.Contains(err.Error(), "load .env") {
		t.Fatalf("Load() error = %v, want load .env error", err)
	}
}

func TestNormalizePEM(t *testing.T) {
	got := NormalizePEM("-----BEGIN KEY-----\\nabc\\n-----END KEY-----\r\n")
	want := "-----BEGIN KEY-----\nabc\n-----END KEY-----"
	if got != want {
		t.Fatalf("NormalizePEM() = %q, want %q", got, want)
	}
}

func TestDecodeSecretKey(t *testing.T) {
	raw, err := json.Marshal(NodePayload{
		CACertPEM:    "-----BEGIN CERTIFICATE-----\\nca\\n-----END CERTIFICATE-----",
		JWTPublicKey: "-----BEGIN PUBLIC KEY-----\\nkey\\n-----END PUBLIC KEY-----",
		NodeCertPEM:  "-----BEGIN CERTIFICATE-----\\nnode\\n-----END CERTIFICATE-----",
		NodeKeyPEM:   "-----BEGIN PRIVATE KEY-----\\nkey\\n-----END PRIVATE KEY-----",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payload, err := DecodeSecretKey(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("DecodeSecretKey() error = %v", err)
	}
	if payload.CACertPEM == "" || payload.JWTPublicKey == "" || payload.NodeCertPEM == "" || payload.NodeKeyPEM == "" {
		t.Fatalf("decoded payload has empty fields")
	}
}

func TestDecodeSecretKeyRequiresFields(t *testing.T) {
	raw, err := json.Marshal(NodePayload{CACertPEM: "cert"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	_, err = DecodeSecretKey(base64.StdEncoding.EncodeToString(raw))
	if err == nil {
		t.Fatalf("DecodeSecretKey() error = nil, want missing field error")
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}

func clearEnv(t *testing.T, names ...string) {
	t.Helper()
	previous := make(map[string]string, len(names))
	present := make(map[string]bool, len(names))
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			previous[name] = value
			present[name] = true
		}
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			var err error
			if present[name] {
				err = os.Setenv(name, previous[name])
			} else {
				err = os.Unsetenv(name)
			}
			if err != nil {
				t.Fatalf("restore %s: %v", name, err)
			}
		}
	})
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
