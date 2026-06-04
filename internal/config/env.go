package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	DefaultNodePort              = 2222
	DefaultInternalRESTPort      = 61001
	DefaultLogLevel              = "info"
	DefaultLogColor              = "always"
	DefaultRWNodeDir             = "/opt/rw-node-go"
	DefaultRequestBodyLimitBytes = int64(1 << 30)
	DefaultNodeTLSClientAuth     = "mtls"
)

type Config struct {
	NodePort                int
	InternalRESTPort        int
	SecretKey               string
	NodeTLSClientAuth       string
	LogLevel                string
	LogColor                string
	RWNodeDir               string
	RequestBodyLimitBytes   int64
	RequireSecretKey        bool
	AllowInsecureHTTPTarget bool
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	nodePort, err := envPort("NODE_PORT", DefaultNodePort)
	if err != nil {
		return Config{}, err
	}
	internalRESTPort, err := envPort("INTERNAL_REST_PORT", DefaultInternalRESTPort)
	if err != nil {
		return Config{}, err
	}
	requestBodyLimitBytes, err := envNonNegativeInt64("REQUEST_BODY_LIMIT_BYTES", DefaultRequestBodyLimitBytes)
	if err != nil {
		return Config{}, err
	}
	requireSecretKey, err := envBool("REQUIRE_SECRET_KEY", false)
	if err != nil {
		return Config{}, err
	}
	allowInsecureHTTPTarget, err := envBool("ALLOW_INSECURE_HTTP_TARGET", true)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		NodePort:                nodePort,
		InternalRESTPort:        internalRESTPort,
		SecretKey:               strings.TrimSpace(os.Getenv("SECRET_KEY")),
		NodeTLSClientAuth:       normalizeNodeTLSClientAuth(envString("NODE_TLS_CLIENT_AUTH", DefaultNodeTLSClientAuth)),
		LogLevel:                normalizeLogLevel(envString("LOG_LEVEL", DefaultLogLevel)),
		LogColor:                normalizeLogColor(envString("LOG_COLOR", DefaultLogColor)),
		RWNodeDir:               envString("RW_NODE_DIR", DefaultRWNodeDir),
		RequestBodyLimitBytes:   requestBodyLimitBytes,
		RequireSecretKey:        requireSecretKey,
		AllowInsecureHTTPTarget: allowInsecureHTTPTarget,
	}

	if cfg.NodePort == cfg.InternalRESTPort {
		return Config{}, fmt.Errorf("NODE_PORT and INTERNAL_REST_PORT must be different")
	}
	if cfg.RequireSecretKey && cfg.SecretKey == "" {
		return Config{}, fmt.Errorf("SECRET_KEY is required")
	}
	if !validNodeTLSClientAuth(cfg.NodeTLSClientAuth) {
		return Config{}, fmt.Errorf("NODE_TLS_CLIENT_AUTH must be one of: mtls, optional, none")
	}
	if !validLogLevel(cfg.LogLevel) {
		return Config{}, fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error")
	}
	if !validLogColor(cfg.LogColor) {
		return Config{}, fmt.Errorf("LOG_COLOR must be one of: always, never")
	}

	return cfg, nil
}

func loadDotEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}

func (c Config) ListenAddress() string {
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(c.NodePort))
}

func (c Config) InternalListenAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(c.InternalRESTPort))
}

func (c Config) TLSClientAuthMode() string {
	mode := normalizeNodeTLSClientAuth(c.NodeTLSClientAuth)
	if mode == "" {
		return DefaultNodeTLSClientAuth
	}
	return mode
}

func (c Config) SlogLevel() slog.Level {
	switch normalizeLogLevel(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (c Config) LogColorEnabled() bool {
	return normalizeLogColor(c.LogColor) != "never"
}

func validNodeTLSClientAuth(value string) bool {
	switch normalizeNodeTLSClientAuth(value) {
	case "mtls", "optional", "none":
		return true
	default:
		return false
	}
}

func normalizeNodeTLSClientAuth(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validLogLevel(value string) bool {
	switch normalizeLogLevel(value) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func normalizeLogLevel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validLogColor(value string) bool {
	switch normalizeLogColor(value) {
	case "always", "never":
		return true
	default:
		return false
	}
}

func normalizeLogColor(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
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

func envBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func envPort(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	if parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", key)
	}
	return parsed, nil
}

func envNonNegativeInt64(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to 0", key)
	}
	return parsed, nil
}
