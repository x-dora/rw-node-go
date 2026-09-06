package xray

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func geodataTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	// TLS because geodata asset URLs must be https, matching the official rule.
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	original := geodataHTTPClient
	geodataHTTPClient = server.Client()
	t.Cleanup(func() { geodataHTTPClient = original })
	return server
}

func geodataConfig(server *httptest.Server, assets ...GeodataAsset) map[string]any {
	items := make([]any, 0, len(assets))
	for _, asset := range assets {
		items = append(items, map[string]any{"url": asset.URL, "file": asset.File})
	}
	return map[string]any{
		"geodata": map[string]any{"assets": items},
	}
}

func TestPrepareGeodataDownloadsAssets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XRAY_LOCATION_ASSET", dir)

	server := geodataTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geoip.dat" {
			_, _ = w.Write([]byte("geoip-content"))
			return
		}
		http.NotFound(w, r)
	})

	config := geodataConfig(server,
		GeodataAsset{URL: server.URL + "/geoip.dat", File: "geoip-custom.dat"},
	)
	PrepareGeodata(context.Background(), config, testLogger())

	content, err := os.ReadFile(filepath.Join(dir, "geoip-custom.dat"))
	if err != nil {
		t.Fatalf("read downloaded asset: %v", err)
	}
	if string(content) != "geoip-content" {
		t.Fatalf("content = %q", content)
	}
}

func TestPrepareGeodataSkipsExistingNonEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XRAY_LOCATION_ASSET", dir)

	if err := os.WriteFile(filepath.Join(dir, "geoip-custom.dat"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	requests := 0
	server := geodataTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte("never-used"))
	})

	config := geodataConfig(server,
		GeodataAsset{URL: server.URL + "/geoip.dat", File: "geoip-custom.dat"},
	)
	PrepareGeodata(context.Background(), config, testLogger())

	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
	content, err := os.ReadFile(filepath.Join(dir, "geoip-custom.dat"))
	if err != nil || string(content) != "existing" {
		t.Fatalf("existing file overwritten: %q %v", content, err)
	}
}

func TestPrepareGeodataCreatesStubOnDownloadFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XRAY_LOCATION_ASSET", dir)

	server := geodataTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	config := geodataConfig(server,
		GeodataAsset{URL: server.URL + "/missing.dat", File: "geosite-custom.dat"},
	)
	PrepareGeodata(context.Background(), config, testLogger())

	info, err := os.Stat(filepath.Join(dir, "geosite-custom.dat"))
	if err != nil {
		t.Fatalf("stub not created: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("stub size = %d, want 0", info.Size())
	}
}

func TestPrepareGeodataIgnoresInvalidAssets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XRAY_LOCATION_ASSET", dir)

	server := geodataTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid asset triggered a download: %s", r.URL.Path)
	})

	config := geodataConfig(server,
		GeodataAsset{URL: "http://insecure.example/geoip.dat", File: "geoip-custom.dat"},
		GeodataAsset{URL: server.URL + "/geoip.dat", File: "../escape.dat"},
		GeodataAsset{URL: server.URL + "/geoip.dat", File: "."},
		GeodataAsset{URL: server.URL + "/geoip.dat", File: "sub/geoip.dat"},
	)
	PrepareGeodata(context.Background(), config, testLogger())

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("dir = %v, want empty", entries)
	}
}

func TestPrepareGeodataIgnoresCoreOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XRAY_LOCATION_ASSET", dir)

	config := map[string]any{
		"geodata": map[string]any{
			"core":   map[string]any{"url": "https://example.com/xray", "sha256": "aa"},
			"assets": []any{},
		},
	}
	PrepareGeodata(context.Background(), config, testLogger())
}

func TestPrepareGeodataWithoutSection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XRAY_LOCATION_ASSET", dir)

	PrepareGeodata(context.Background(), map[string]any{"inbounds": []any{}}, testLogger())
	PrepareGeodata(context.Background(), nil, testLogger())
}
