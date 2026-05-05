package config

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

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
	if cfg.InternalSocketPath != DefaultInternalSocketPath {
		t.Fatalf("InternalSocketPath = %q, want %q", cfg.InternalSocketPath, DefaultInternalSocketPath)
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
