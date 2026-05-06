package testkit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	PanelBaseURLEnv = "PANEL_BASE_URL"
	PanelAPIKeyEnv  = "PANEL_API_KEY"
	PanelNodeIDEnv  = "PANEL_NODE_ID"
)

type PanelClientConfig struct {
	BaseURL    string
	APIKey     string
	NodeID     string
	HTTPClient *http.Client
	Logger     *slog.Logger
}

type PanelClient struct {
	baseURL    *url.URL
	apiKey     string
	nodeID     string
	httpClient *http.Client
	logger     *slog.Logger
}

type PanelRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   any
}

type PanelResponse struct {
	Method        string
	URL           string
	StatusCode    int
	Duration      time.Duration
	Body          []byte
	PrettyBody    string
	ContentType   string
	ErrorCategory string
}

type NodeStatusSummary struct {
	UUID              string `json:"uuid"`
	Name              string `json:"name"`
	Address           string `json:"address"`
	Port              any    `json:"port"`
	IsConnected       bool   `json:"is_connected"`
	IsConnecting      bool   `json:"is_connecting"`
	IsDisabled        bool   `json:"is_disabled"`
	LastStatusChange  any    `json:"last_status_change"`
	LastStatusMessage any    `json:"last_status_message"`
	CountryCode       string `json:"country_code"`
}

func NewPanelClient(cfg PanelClientConfig) (*PanelClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("%s is required", PanelBaseURLEnv)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", PanelBaseURLEnv, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%s must use http or https", PanelBaseURLEnv)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("%s must include a host", PanelBaseURLEnv)
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%s is required", PanelAPIKeyEnv)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	return &PanelClient{
		baseURL:    parsed,
		apiKey:     apiKey,
		nodeID:     strings.TrimSpace(cfg.NodeID),
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

func NewPanelClientFromEnv(logger *slog.Logger) (*PanelClient, error) {
	return NewPanelClient(PanelClientConfig{
		BaseURL: os.Getenv(PanelBaseURLEnv),
		APIKey:  os.Getenv(PanelAPIKeyEnv),
		NodeID:  os.Getenv(PanelNodeIDEnv),
		Logger:  logger,
	})
}

func (c *PanelClient) NodeID() string {
	return c.nodeID
}

func (c *PanelClient) ConfigSummary() map[string]any {
	return map[string]any{
		"panel_base_url": c.baseURL.String(),
		"panel_api_key":  RedactSecret(c.apiKey),
		"panel_node_id":  emptyAsNull(c.nodeID),
	}
}

func (c *PanelClient) Do(ctx context.Context, req PanelRequest) (*PanelResponse, error) {
	if strings.TrimSpace(req.Method) == "" {
		return nil, fmt.Errorf("panel request method is required")
	}
	target, err := c.resolveURL(req.Path, req.Query)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	if req.Body != nil {
		data, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("marshal panel request body: %w", err)
		}
		body = bytes.NewReader(data)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create panel request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	started := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	duration := time.Since(started)
	if err != nil {
		category := ClassifyHTTPError(err, 0)
		c.logRequest(req.Method, target, 0, duration, category, nil)
		return nil, fmt.Errorf("%s: %w", category, err)
	}
	defer httpResp.Body.Close()

	data, readErr := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	resp := &PanelResponse{
		Method:        req.Method,
		URL:           target.String(),
		StatusCode:    httpResp.StatusCode,
		Duration:      duration,
		Body:          data,
		PrettyBody:    PrettyJSON(data),
		ContentType:   httpResp.Header.Get("Content-Type"),
		ErrorCategory: ClassifyHTTPError(readErr, httpResp.StatusCode),
	}
	c.logRequest(req.Method, target, resp.StatusCode, duration, resp.ErrorCategory, data)

	if readErr != nil {
		return resp, fmt.Errorf("read panel response: %w", readErr)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return resp, fmt.Errorf("panel request failed: status=%d category=%s", httpResp.StatusCode, resp.ErrorCategory)
	}
	return resp, nil
}

func (c *PanelClient) Get(ctx context.Context, path string) (*PanelResponse, error) {
	return c.Do(ctx, PanelRequest{Method: http.MethodGet, Path: path})
}

func (c *PanelClient) Post(ctx context.Context, path string, body any) (*PanelResponse, error) {
	return c.Do(ctx, PanelRequest{Method: http.MethodPost, Path: path, Body: body})
}

func (c *PanelClient) GetNodeStatusSummary(ctx context.Context) ([]NodeStatusSummary, *PanelResponse, error) {
	resp, err := c.Get(ctx, "/api/nodes")
	if err != nil {
		return nil, resp, err
	}

	var payload struct {
		Response []map[string]any `json:"response"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, resp, fmt.Errorf("decode panel nodes response: %w", err)
	}

	summaries := make([]NodeStatusSummary, 0, len(payload.Response))
	for _, item := range payload.Response {
		summaries = append(summaries, NodeStatusSummary{
			UUID:              stringValue(item["uuid"]),
			Name:              stringValue(item["name"]),
			Address:           stringValue(item["address"]),
			Port:              item["port"],
			IsConnected:       boolValue(item["isConnected"]),
			IsConnecting:      boolValue(item["isConnecting"]),
			IsDisabled:        boolValue(item["isDisabled"]),
			LastStatusChange:  item["lastStatusChange"],
			LastStatusMessage: item["lastStatusMessage"],
			CountryCode:       stringValue(item["countryCode"]),
		})
	}

	return summaries, resp, nil
}

func (c *PanelClient) GetNodeStatusSummaryByUUID(ctx context.Context, nodeUUID string) (*NodeStatusSummary, *PanelResponse, error) {
	nodeUUID = strings.TrimSpace(nodeUUID)
	if nodeUUID == "" {
		return nil, nil, fmt.Errorf("panel node uuid is required")
	}
	if !isUUID(nodeUUID) {
		return nil, nil, fmt.Errorf("panel node uuid must be a valid uuid: %q", nodeUUID)
	}

	resp, err := c.Get(ctx, "/api/nodes/"+url.PathEscape(nodeUUID))
	if err != nil {
		return nil, resp, err
	}

	var payload struct {
		Response map[string]any `json:"response"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, resp, fmt.Errorf("decode panel node response: %w", err)
	}

	summary := nodeStatusSummaryFromMap(payload.Response)
	if summary.UUID == "" && summary.Name == "" && summary.Address == "" {
		return nil, resp, fmt.Errorf("panel node response missing expected fields")
	}
	return &summary, resp, nil
}

func (c *PanelClient) GetNodeStatusSummaryByHint(ctx context.Context, nodeHint string) (*NodeStatusSummary, *PanelResponse, error) {
	nodeHint = strings.TrimSpace(nodeHint)
	if nodeHint == "" {
		return nil, nil, fmt.Errorf("panel node hint is required")
	}
	if isUUID(nodeHint) {
		return c.GetNodeStatusSummaryByUUID(ctx, nodeHint)
	}

	summaries, resp, err := c.GetNodeStatusSummary(ctx)
	if err != nil {
		return nil, resp, err
	}

	hintLower := strings.ToLower(nodeHint)
	matches := make([]NodeStatusSummary, 0, 3)
	for _, summary := range summaries {
		if nodeSummaryMatchesHint(summary, hintLower) {
			matches = append(matches, summary)
		}
	}

	switch len(matches) {
	case 0:
		return nil, resp, fmt.Errorf("no panel node matched hint %q", nodeHint)
	case 1:
		return &matches[0], resp, nil
	default:
		return nil, resp, fmt.Errorf("multiple panel nodes matched hint %q", nodeHint)
	}
}

func (c *PanelClient) EnableNode(ctx context.Context, nodeUUID string) (*NodeStatusSummary, *PanelResponse, error) {
	return c.nodeAction(ctx, nodeUUID, "enable")
}

func (c *PanelClient) DisableNode(ctx context.Context, nodeUUID string) (*NodeStatusSummary, *PanelResponse, error) {
	return c.nodeAction(ctx, nodeUUID, "disable")
}

func (c *PanelClient) nodeAction(ctx context.Context, nodeUUID string, action string) (*NodeStatusSummary, *PanelResponse, error) {
	nodeUUID = strings.TrimSpace(nodeUUID)
	if nodeUUID == "" {
		return nil, nil, fmt.Errorf("panel node uuid is required")
	}
	if !isUUID(nodeUUID) {
		return nil, nil, fmt.Errorf("panel node uuid must be a valid uuid: %q", nodeUUID)
	}

	resp, err := c.Post(ctx, "/api/nodes/"+url.PathEscape(nodeUUID)+"/actions/"+action, nil)
	if err != nil {
		return nil, resp, err
	}

	var payload struct {
		Response map[string]any `json:"response"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, resp, fmt.Errorf("decode panel node %s response: %w", action, err)
	}

	summary := nodeStatusSummaryFromMap(payload.Response)
	if summary.UUID == "" && summary.Name == "" && summary.Address == "" {
		return nil, resp, fmt.Errorf("panel node %s response missing expected fields", action)
	}
	return &summary, resp, nil
}

func (c *PanelClient) resolveURL(path string, query url.Values) (*url.URL, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("panel request path is required")
	}

	resolved := *c.baseURL
	basePath := strings.TrimRight(resolved.Path, "/")
	requestPath := "/" + strings.TrimLeft(path, "/")
	resolved.Path = basePath + requestPath
	if query != nil {
		resolved.RawQuery = query.Encode()
	}
	return &resolved, nil
}

func (c *PanelClient) logRequest(method string, target *url.URL, status int, duration time.Duration, category string, body []byte) {
	bodyHash := ""
	bodyBytes := len(body)
	if bodyBytes > 0 {
		sum := sha256.Sum256(body)
		bodyHash = hex.EncodeToString(sum[:8])
	}
	c.logger.Info("panel request",
		"method", method,
		"url", RedactURL(target.String()),
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"category", category,
		"response_bytes", bodyBytes,
		"response_sha256_8", bodyHash,
	)
}

func ClassifyHTTPError(err error, status int) string {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "timeout"
		}
		var netErr interface{ Timeout() bool }
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "timeout"
		}
		return "network_error"
	}
	switch {
	case status == 0:
		return "not_sent"
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "auth_error"
	case status == http.StatusNotFound:
		return "not_found"
	case status >= 500:
		return "panel_server_error"
	case status >= 400:
		return "panel_client_error"
	default:
		return "ok"
	}
}

func PrettyJSON(data []byte) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return RedactText(strings.TrimSpace(string(data)))
	}
	out, err := json.MarshalIndent(RedactJSON(decoded), "", "  ")
	if err != nil {
		return RedactText(strings.TrimSpace(string(data)))
	}
	return string(out)
}

func RedactJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveKey(key) {
				if text, ok := item.(string); ok {
					redacted[key] = RedactSecret(text)
				} else {
					redacted[key] = "[REDACTED]"
				}
				continue
			}
			redacted[key] = RedactJSON(item)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for i, item := range typed {
			redacted[i] = RedactJSON(item)
		}
		return redacted
	case string:
		return RedactText(typed)
	default:
		return value
	}
}

func RedactSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "[REDACTED]"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func RedactText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = pemBlockPattern.ReplaceAllString(value, "[REDACTED_PEM]")
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = jwtPattern.ReplaceAllString(value, "[REDACTED_JWT]")
	value = keyValuePattern.ReplaceAllString(value, `${1}${2}[REDACTED]`)
	return value
}

func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return RedactText(raw)
	}
	query := parsed.Query()
	for key := range query {
		if isSensitiveKey(key) {
			query.Set(key, "[REDACTED]")
		}
	}
	parsed.RawQuery = query.Encode()
	if parsed.User != nil {
		parsed.User = url.User("[REDACTED]")
	}
	return parsed.String()
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), ".", "_"))
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "private_key") ||
		strings.Contains(normalized, "node_key") ||
		strings.Contains(normalized, "cert_pem")
}

func emptyAsNull(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nodeStatusSummaryFromMap(item map[string]any) NodeStatusSummary {
	return NodeStatusSummary{
		UUID:              stringValue(item["uuid"]),
		Name:              stringValue(item["name"]),
		Address:           stringValue(item["address"]),
		Port:              item["port"],
		IsConnected:       boolValue(item["isConnected"]),
		IsConnecting:      boolValue(item["isConnecting"]),
		IsDisabled:        boolValue(item["isDisabled"]),
		LastStatusChange:  item["lastStatusChange"],
		LastStatusMessage: item["lastStatusMessage"],
		CountryCode:       stringValue(item["countryCode"]),
	}
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func boolValue(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var (
	pemBlockPattern = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]+-----.*?-----END [A-Z0-9 ]+-----`)
	bearerPattern   = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/\-]+=*`)
	jwtPattern      = regexp.MustCompile(`\b[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	keyValuePattern = regexp.MustCompile(`(?i)\b(secret[_-]?key|api[_-]?key|apikey|token|password|private[_-]?key|node[_-]?key|cert[_-]?pem|authorization)(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s"',;}&]+)`)
)

func IsUUID(value string) bool {
	return isUUID(strings.TrimSpace(value))
}

func isUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

func nodeSummaryMatchesHint(summary NodeStatusSummary, hintLower string) bool {
	if hintLower == "" {
		return false
	}
	if strings.EqualFold(summary.UUID, hintLower) || strings.EqualFold(summary.Name, hintLower) || strings.EqualFold(summary.Address, hintLower) {
		return true
	}
	if strings.Contains(strings.ToLower(summary.UUID), hintLower) {
		return true
	}
	if strings.Contains(strings.ToLower(summary.Name), hintLower) {
		return true
	}
	if strings.Contains(strings.ToLower(summary.Address), hintLower) {
		return true
	}
	return false
}
