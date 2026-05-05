package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("NODE_PORT", "")
	t.Setenv("XTLS_API_PORT", "")
	t.Setenv("RW_NODE_DIR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.NodePort != DefaultNodePort {
		t.Fatalf("NodePort = %d, want %d", cfg.NodePort, DefaultNodePort)
	}
	if cfg.XTLSAPIPort != DefaultXTLSAPIPort {
		t.Fatalf("XTLSAPIPort = %d, want %d", cfg.XTLSAPIPort, DefaultXTLSAPIPort)
	}
	if cfg.RWNodeDir != DefaultRWNodeDir {
		t.Fatalf("RWNodeDir = %q, want %q", cfg.RWNodeDir, DefaultRWNodeDir)
	}
}

func TestNormalizePEM(t *testing.T) {
	got := NormalizePEM("-----BEGIN KEY-----\\nabc\\n-----END KEY-----\r\n")
	want := "-----BEGIN KEY-----\nabc\n-----END KEY-----"
	if got != want {
		t.Fatalf("NormalizePEM() = %q, want %q", got, want)
	}
}
