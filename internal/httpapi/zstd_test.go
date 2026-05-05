package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestZstdMiddleware(t *testing.T) {
	handler := ZstdMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		_, _ = w.Write(body)
	}))

	var compressed bytes.Buffer
	encoder, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatalf("new zstd writer: %v", err)
	}
	if _, err := encoder.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("write zstd body: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close zstd writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "zstd")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestZstdMiddlewareRejectsInvalidBody(t *testing.T) {
	handler := ZstdMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not-zstd"))
	req.Header.Set("Content-Encoding", "zstd")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestZstdMiddlewareEnforcesDecodedLimit(t *testing.T) {
	var compressed bytes.Buffer
	encoder, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatalf("new zstd writer: %v", err)
	}
	if _, err := encoder.Write([]byte("123456")); err != nil {
		t.Fatalf("write zstd body: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close zstd writer: %v", err)
	}

	handler := ZstdMiddlewareWithLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), 5)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "zstd")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
