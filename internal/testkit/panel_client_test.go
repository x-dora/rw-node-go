package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewPanelClientRequiresConfig(t *testing.T) {
	_, err := NewPanelClient(PanelClientConfig{})
	if err == nil || !strings.Contains(err.Error(), PanelBaseURLEnv) {
		t.Fatalf("NewPanelClient() error = %v, want missing base URL", err)
	}

	_, err = NewPanelClient(PanelClientConfig{BaseURL: "https://panel.example"})
	if err == nil || !strings.Contains(err.Error(), PanelAPIKeyEnv) {
		t.Fatalf("NewPanelClient() error = %v, want missing API key", err)
	}
}

func TestPrettyJSONRedactsSensitiveFields(t *testing.T) {
	input := []byte(`{
		"apiKey": "abcdefghijklmnopqrstuvwxyz",
		"nested": {
			"secretKey": "1234567890abcdef",
			"plain": "visible",
			"message": "Authorization: Bearer aaaaaaaaaa.bbbbbbbbbb.cccccccccc"
		},
		"items": [{"token": "short"}]
	}`)

	got := PrettyJSON(input)
	if strings.Contains(got, "abcdefghijklmnopqrstuvwxyz") ||
		strings.Contains(got, "1234567890abcdef") ||
		strings.Contains(got, "short") ||
		strings.Contains(got, "aaaaaaaaaa.bbbbbbbbbb.cccccccccc") {
		t.Fatalf("PrettyJSON() leaked sensitive data: %s", got)
	}
	if !strings.Contains(got, "visible") {
		t.Fatalf("PrettyJSON() removed non-sensitive data: %s", got)
	}
}

func TestPrettyJSONRedactsNonJSONBody(t *testing.T) {
	input := []byte("request failed: Authorization: Bearer abc.def.ghi token=super-secret")

	got := PrettyJSON(input)
	if strings.Contains(got, "abc.def.ghi") || strings.Contains(got, "super-secret") {
		t.Fatalf("PrettyJSON() leaked non-JSON sensitive data: %s", got)
	}
	if !strings.Contains(got, "request failed") {
		t.Fatalf("PrettyJSON() removed safe context: %s", got)
	}
}

func TestRedactText(t *testing.T) {
	jwt := "aaaaaaaaaa.bbbbbbbbbb.cccccccccc"
	input := `Authorization: Bearer ` + jwt + `
secretKey=very-secret-value
token="quoted-secret"
-----BEGIN PRIVATE KEY-----
private-key-material
-----END PRIVATE KEY-----
plain text`

	got := RedactText(input)
	for _, leaked := range []string{jwt, "very-secret-value", "quoted-secret", "private-key-material"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("RedactText() leaked %q in %s", leaked, got)
		}
	}
	if !strings.Contains(got, "plain text") {
		t.Fatalf("RedactText() removed safe context: %s", got)
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://user:pass@panel.example/api?token=secret&node=visible")
	if strings.Contains(got, "secret") || strings.Contains(got, "user:pass") {
		t.Fatalf("RedactURL() leaked sensitive data: %s", got)
	}
	if !strings.Contains(got, "node=visible") {
		t.Fatalf("RedactURL() removed safe query data: %s", got)
	}
}

func TestPanelClientDoLogsSummary(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Api-Key"); got != "" {
			t.Fatalf("X-Api-Key = %q, want empty", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "token": "do-not-log"})
	}))
	defer server.Close()

	client, err := NewPanelClient(PanelClientConfig{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Logger:  logger,
	})
	if err != nil {
		t.Fatalf("NewPanelClient() error = %v", err)
	}

	resp, err := client.Get(context.Background(), "/api/health")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if strings.Contains(resp.PrettyBody, "do-not-log") {
		t.Fatalf("PrettyBody leaked token: %s", resp.PrettyBody)
	}

	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, `"category":"ok"`) {
		t.Fatalf("log output missing category: %s", logOutput)
	}
	if strings.Contains(logOutput, "test-api-key") || strings.Contains(logOutput, "do-not-log") {
		t.Fatalf("log output leaked sensitive data: %s", logOutput)
	}
}
