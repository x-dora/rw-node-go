package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteHTTPEnvelope(rec, http.StatusOK, map[string]any{"success": true})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !body["response"]["success"] {
		t.Fatalf("response.success = false, want true")
	}
}
