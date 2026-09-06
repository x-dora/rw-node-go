package xray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

const (
	// GeodataDownloadTimeout matches the official TOTAL_TIMEOUT_MS per asset.
	GeodataDownloadTimeout = 15 * time.Second
	// geodataConcurrency matches the official GeodataService concurrency.
	geodataConcurrency = 5
	// maxGeodataAssetSize bounds a single asset download.
	maxGeodataAssetSize = 256 << 20
)

// geodataFileNamePattern mirrors the official FILE_NAME_REGEX: a plain file
// name without path separators.
var geodataFileNamePattern = regexp.MustCompile(`^[\w.-]+$`)

type GeodataAsset struct {
	URL  string `json:"url"`
	File string `json:"file"`
}

// geodataHTTPClient is an indirection for tests (httptest TLS servers need a
// trusting client); production uses the default client.
var geodataHTTPClient = http.DefaultClient

// PrepareGeodata downloads the assets listed under config["geodata"].assets
// into the xray asset directory, mirroring the official GeodataService
// (node 3.1.0+): existing non-empty files are skipped, failed downloads leave
// an empty stub file so xray-core can still start, and the "core" section is
// ignored because the embedded xray-core cannot be swapped at runtime.
// Failures never abort the start flow.
func PrepareGeodata(ctx context.Context, panelConfig map[string]any, logger *slog.Logger) {
	if panelConfig == nil {
		return
	}
	section, ok := panelConfig["geodata"].(map[string]any)
	if !ok {
		return
	}

	if _, hasCore := section["core"]; hasCore {
		logger.Warn("geodata core override is not supported: the embedded xray-core binary is fixed")
	}

	raw, err := json.Marshal(section)
	if err != nil {
		logger.Warn("invalid geodata section, skipped", "error", err)
		return
	}
	var parsed struct {
		Assets []GeodataAsset `json:"assets"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		logger.Warn("invalid geodata section, skipped", "error", err)
		return
	}

	assets := make([]GeodataAsset, 0, len(parsed.Assets))
	for _, asset := range parsed.Assets {
		if !validGeodataAsset(asset) {
			logger.Warn("ignoring invalid geodata asset", "file", asset.File, "url", asset.URL)
			continue
		}
		assets = append(assets, asset)
	}
	if len(assets) == 0 {
		return
	}

	assetDir := config.XrayAssetDir()
	started := time.Now()

	sem := make(chan struct{}, geodataConcurrency)
	var wg sync.WaitGroup
	for _, asset := range assets {
		wg.Add(1)
		go func(asset GeodataAsset) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			prepareGeodataAsset(ctx, assetDir, asset, logger)
		}(asset)
	}
	wg.Wait()

	logger.Info("geodata assets processed",
		"count", len(assets),
		"dir", assetDir,
		"duration", time.Since(started).Round(time.Millisecond),
	)
}

func validGeodataAsset(asset GeodataAsset) bool {
	parsed, err := url.Parse(asset.URL)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	if !geodataFileNamePattern.MatchString(asset.File) || asset.File == "." || asset.File == ".." {
		return false
	}
	return true
}

func prepareGeodataAsset(ctx context.Context, assetDir string, asset GeodataAsset, logger *slog.Logger) {
	path := filepath.Join(assetDir, asset.File)

	if exists, err := fileExistsNonEmpty(path); err == nil && exists {
		return
	}

	if err := downloadGeodataAsset(ctx, asset.URL, path); err != nil {
		logger.Error("failed to download geodata asset", "file", asset.File, "error", err)
		createGeodataStub(path, logger)
		return
	}
	logger.Info("downloaded geodata asset", "file", asset.File)
}

func downloadGeodataAsset(ctx context.Context, downloadURL string, path string) error {
	ctx, cancel := context.WithTimeout(ctx, GeodataDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := geodataHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".geodata-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, maxGeodataAssetSize)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func createGeodataStub(path string, logger *slog.Logger) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Error("failed to create geodata stub directory", "dir", filepath.Dir(path), "error", err)
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			logger.Error("failed to create geodata stub", "file", path, "error", err)
		}
		return
	}
	file.Close()
	logger.Warn("created empty stub geodata asset", "file", path)
}

func fileExistsNonEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return info.Mode().IsRegular() && info.Size() > 0, nil
}
