package config

import (
	"os"
	"strings"
)

// DefaultXrayAssetDir matches the official node image asset location.
const DefaultXrayAssetDir = "/usr/local/share/xray"

type Paths struct {
	RWNodeDir string
}

func (c Config) Paths() Paths {
	return Paths{
		RWNodeDir: c.RWNodeDir,
	}
}

// XrayAssetDir resolves the xray asset directory from the environment:
// XRAY_LOCATION_ASSET first (xray-core reads this), then XRAY_ASSET_DIR
// (local dev convenience), then the default share location.
func XrayAssetDir() string {
	if dir := strings.TrimSpace(os.Getenv("XRAY_LOCATION_ASSET")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("XRAY_ASSET_DIR")); dir != "" {
		return dir
	}
	return DefaultXrayAssetDir
}
