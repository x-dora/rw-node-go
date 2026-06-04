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
		"LOG_COLOR",
		"REQUEST_BODY_LIMIT_BYTES",
		"REQUIRE_SECRET_KEY",
		"ALLOW_INSECURE_HTTP_TARGET",
		"NODE_TLS_CLIENT_AUTH",
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
	if cfg.LogLevel != DefaultLogLevel {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
	if cfg.SlogLevel().String() != "INFO" {
		t.Fatalf("SlogLevel() = %s, want INFO", cfg.SlogLevel())
	}
	if cfg.LogColor != DefaultLogColor {
		t.Fatalf("LogColor = %q, want %q", cfg.LogColor, DefaultLogColor)
	}
	if !cfg.LogColorEnabled() {
		t.Fatalf("LogColorEnabled() = false, want true")
	}
	if cfg.TLSClientAuthMode() != DefaultNodeTLSClientAuth {
		t.Fatalf("TLSClientAuthMode() = %q, want %q", cfg.TLSClientAuthMode(), DefaultNodeTLSClientAuth)
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
		"LOG_COLOR",
		"REQUEST_BODY_LIMIT_BYTES",
		"REQUIRE_SECRET_KEY",
		"ALLOW_INSECURE_HTTP_TARGET",
		"NODE_TLS_CLIENT_AUTH",
	)
	writeFile(t, filepath.Join(dir, ".env"), strings.Join([]string{
		"NODE_PORT=3333",
		"INTERNAL_REST_PORT=62000",
		"LOG_LEVEL=debug",
		"LOG_COLOR=never",
		"SECRET_KEY=dotenv-secret",
		"NODE_TLS_CLIENT_AUTH=none",
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
	if cfg.SlogLevel().String() != "DEBUG" {
		t.Fatalf("SlogLevel() = %s, want DEBUG", cfg.SlogLevel())
	}
	if cfg.LogColor != "never" {
		t.Fatalf("LogColor = %q, want never", cfg.LogColor)
	}
	if cfg.LogColorEnabled() {
		t.Fatalf("LogColorEnabled() = true, want false")
	}
	if cfg.SecretKey != "dotenv-secret" {
		t.Fatalf("SecretKey was not loaded from .env")
	}
	if cfg.TLSClientAuthMode() != "none" {
		t.Fatalf("TLSClientAuthMode() = %q, want none", cfg.TLSClientAuthMode())
	}
}

func TestLoadEnvOverridesDotEnv(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearEnv(t, "INTERNAL_REST_PORT", "SECRET_KEY", "RW_NODE_DIR", "LOG_LEVEL", "LOG_COLOR", "REQUEST_BODY_LIMIT_BYTES", "REQUIRE_SECRET_KEY", "ALLOW_INSECURE_HTTP_TARGET", "NODE_TLS_CLIENT_AUTH")
	t.Setenv("NODE_PORT", "4444")
	t.Setenv("LOG_COLOR", "never")
	writeFile(t, filepath.Join(dir, ".env"), "NODE_PORT=3333\nLOG_COLOR=always\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.NodePort != 4444 {
		t.Fatalf("NodePort = %d, want 4444", cfg.NodePort)
	}
	if cfg.LogColor != "never" {
		t.Fatalf("LogColor = %q, want never", cfg.LogColor)
	}
}

func TestLoadRejectsMalformedDotEnv(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearEnv(t, "NODE_PORT", "INTERNAL_REST_PORT", "SECRET_KEY", "RW_NODE_DIR", "LOG_LEVEL", "LOG_COLOR", "REQUEST_BODY_LIMIT_BYTES", "REQUIRE_SECRET_KEY", "ALLOW_INSECURE_HTTP_TARGET", "NODE_TLS_CLIENT_AUTH")
	writeFile(t, filepath.Join(dir, ".env"), "NODE_PORT=\"unterminated\n")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load() error = nil, want malformed .env error")
	}
	if !strings.Contains(err.Error(), "load .env") {
		t.Fatalf("Load() error = %v, want load .env error", err)
	}
}

func TestLoadRejectsInvalidNodeTLSClientAuth(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearEnv(t, "NODE_PORT", "INTERNAL_REST_PORT", "SECRET_KEY", "RW_NODE_DIR", "LOG_LEVEL", "LOG_COLOR", "REQUEST_BODY_LIMIT_BYTES", "REQUIRE_SECRET_KEY", "ALLOW_INSECURE_HTTP_TARGET", "NODE_TLS_CLIENT_AUTH")
	writeFile(t, filepath.Join(dir, ".env"), "NODE_TLS_CLIENT_AUTH=disabled\n")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load() error = nil, want invalid NODE_TLS_CLIENT_AUTH error")
	}
	if !strings.Contains(err.Error(), "NODE_TLS_CLIENT_AUTH") {
		t.Fatalf("Load() error = %v, want NODE_TLS_CLIENT_AUTH error", err)
	}
}

func TestLoadRejectsInvalidNumericAndBooleanEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		contains string
	}{
		{name: "node port is not integer", key: "NODE_PORT", value: "abc", contains: "NODE_PORT"},
		{name: "internal port is not integer", key: "INTERNAL_REST_PORT", value: "abc", contains: "INTERNAL_REST_PORT"},
		{name: "request body limit is not integer", key: "REQUEST_BODY_LIMIT_BYTES", value: "abc", contains: "REQUEST_BODY_LIMIT_BYTES"},
		{name: "require secret key is not bool", key: "REQUIRE_SECRET_KEY", value: "sometimes", contains: "REQUIRE_SECRET_KEY"},
		{name: "allow insecure target is not bool", key: "ALLOW_INSECURE_HTTP_TARGET", value: "sometimes", contains: "ALLOW_INSECURE_HTTP_TARGET"},
		{name: "log level is invalid", key: "LOG_LEVEL", value: "trace", contains: "LOG_LEVEL"},
		{name: "log color is auto", key: "LOG_COLOR", value: "auto", contains: "LOG_COLOR"},
		{name: "log color is off", key: "LOG_COLOR", value: "off", contains: "LOG_COLOR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			chdir(t, dir)
			clearEnv(t,
				"NODE_PORT",
				"INTERNAL_REST_PORT",
				"SECRET_KEY",
				"RW_NODE_DIR",
				"LOG_LEVEL",
				"LOG_COLOR",
				"REQUEST_BODY_LIMIT_BYTES",
				"REQUIRE_SECRET_KEY",
				"ALLOW_INSECURE_HTTP_TARGET",
				"NODE_TLS_CLIENT_AUTH",
			)
			t.Setenv(tt.key, tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want %s error", tt.key)
			}
			if !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("Load() error = %v, want %q", err, tt.contains)
			}
		})
	}
}

func TestLoadRejectsInvalidPortsAndNegativeBodyLimit(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		contains string
	}{
		{name: "node port below range", env: map[string]string{"NODE_PORT": "0"}, contains: "NODE_PORT"},
		{name: "node port above range", env: map[string]string{"NODE_PORT": "65536"}, contains: "NODE_PORT"},
		{name: "internal port below range", env: map[string]string{"INTERNAL_REST_PORT": "0"}, contains: "INTERNAL_REST_PORT"},
		{name: "same ports", env: map[string]string{"NODE_PORT": "3333", "INTERNAL_REST_PORT": "3333"}, contains: "must be different"},
		{name: "negative body limit", env: map[string]string{"REQUEST_BODY_LIMIT_BYTES": "-1"}, contains: "REQUEST_BODY_LIMIT_BYTES"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			chdir(t, dir)
			clearEnv(t,
				"NODE_PORT",
				"INTERNAL_REST_PORT",
				"SECRET_KEY",
				"RW_NODE_DIR",
				"LOG_LEVEL",
				"LOG_COLOR",
				"REQUEST_BODY_LIMIT_BYTES",
				"REQUIRE_SECRET_KEY",
				"ALLOW_INSECURE_HTTP_TARGET",
				"NODE_TLS_CLIENT_AUTH",
			)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want %s", tt.contains)
			}
			if !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("Load() error = %v, want %q", err, tt.contains)
			}
		})
	}
}

func TestLoadAllowsUnlimitedRequestBodyLimit(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearEnv(t,
		"NODE_PORT",
		"INTERNAL_REST_PORT",
		"SECRET_KEY",
		"RW_NODE_DIR",
		"LOG_LEVEL",
		"LOG_COLOR",
		"REQUEST_BODY_LIMIT_BYTES",
		"REQUIRE_SECRET_KEY",
		"ALLOW_INSECURE_HTTP_TARGET",
		"NODE_TLS_CLIENT_AUTH",
	)
	t.Setenv("REQUEST_BODY_LIMIT_BYTES", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RequestBodyLimitBytes != 0 {
		t.Fatalf("RequestBodyLimitBytes = %d, want 0", cfg.RequestBodyLimitBytes)
	}
}

func TestTLSClientAuthModeTrimsManualConfig(t *testing.T) {
	cfg := Config{NodeTLSClientAuth: " NoNe "}
	if cfg.TLSClientAuthMode() != "none" {
		t.Fatalf("TLSClientAuthMode() = %q, want none", cfg.TLSClientAuthMode())
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
