package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultNodePort              = 2222
	DefaultInternalRESTPort      = 61001
	DefaultLogLevel              = "info"
	DefaultRWNodeDir             = "/opt/rw-node-go"
	DefaultRequestBodyLimitBytes = int64(1 << 30)
)

type Config struct {
	NodePort                int
	InternalRESTPort        int
	SecretKey               string
	LogLevel                string
	RWNodeDir               string
	RequestBodyLimitBytes   int64
	RequireSecretKey        bool
	AllowInsecureHTTPTarget bool
}

func Load() (Config, error) {
	cfg := Config{
		NodePort:              envInt("NODE_PORT", DefaultNodePort),
		InternalRESTPort:      envInt("INTERNAL_REST_PORT", DefaultInternalRESTPort),
		SecretKey:             strings.TrimSpace(os.Getenv("SECRET_KEY")),
		LogLevel:              envString("LOG_LEVEL", DefaultLogLevel),
		RWNodeDir:             envString("RW_NODE_DIR", DefaultRWNodeDir),
		RequestBodyLimitBytes: envInt64("REQUEST_BODY_LIMIT_BYTES", DefaultRequestBodyLimitBytes),
		RequireSecretKey:      envBool("REQUIRE_SECRET_KEY", false),
		AllowInsecureHTTPTarget: envBool(
			"ALLOW_INSECURE_HTTP_TARGET",
			true,
		),
	}

	if cfg.RequireSecretKey && cfg.SecretKey == "" {
		return Config{}, fmt.Errorf("SECRET_KEY is required")
	}

	return cfg, nil
}

func (c Config) ListenAddress() string {
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(c.NodePort))
}

func (c Config) InternalListenAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(c.InternalRESTPort))
}

type NodePayload struct {
	CACertPEM    string `json:"caCertPem"`
	JWTPublicKey string `json:"jwtPublicKey"`
	NodeCertPEM  string `json:"nodeCertPem"`
	NodeKeyPEM   string `json:"nodeKeyPem"`
}

func DecodeSecretKey(secretKey string) (NodePayload, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(secretKey))
	if err != nil {
		return NodePayload{}, fmt.Errorf("decode SECRET_KEY: %w", err)
	}

	var payload NodePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return NodePayload{}, fmt.Errorf("parse SECRET_KEY payload: %w", err)
	}

	payload.CACertPEM = NormalizePEM(payload.CACertPEM)
	payload.JWTPublicKey = NormalizePEM(payload.JWTPublicKey)
	payload.NodeCertPEM = NormalizePEM(payload.NodeCertPEM)
	payload.NodeKeyPEM = NormalizePEM(payload.NodeKeyPEM)

	if payload.CACertPEM == "" {
		return NodePayload{}, fmt.Errorf("SECRET_KEY payload missing caCertPem")
	}
	if payload.JWTPublicKey == "" {
		return NodePayload{}, fmt.Errorf("SECRET_KEY payload missing jwtPublicKey")
	}
	if payload.NodeCertPEM == "" {
		return NodePayload{}, fmt.Errorf("SECRET_KEY payload missing nodeCertPem")
	}
	if payload.NodeKeyPEM == "" {
		return NodePayload{}, fmt.Errorf("SECRET_KEY payload missing nodeKeyPem")
	}

	return payload, nil
}

func NormalizePEM(value string) string {
	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSpace(value)

	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !lastBlank {
				normalized = append(normalized, "")
			}
			lastBlank = true
			continue
		}
		normalized = append(normalized, line)
		lastBlank = false
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
