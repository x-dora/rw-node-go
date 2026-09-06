package logview

import (
	"fmt"
	"os"
	"runtime"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/version"
)

func StartupSummary(cfg config.Config, nodeVersion string) string {
	return Table("rw-node-go starting",
		Field("Project Version", version.ProjectVersion),
		Field("Panel Node Version", nodeVersion),
		Field("Commit", version.Commit),
		Field("Build Date", version.BuildDate),
		Field("Go Version", runtime.Version()),
		Field("OS/Arch", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)),
		Field("PID", os.Getpid()),
		Field("Runtime Mode", "embedded xray-core"),
		Field("Main Listen", cfg.ListenAddress()),
		Field("Internal Listen", cfg.InternalListenAddress()),
		Field("TLS Enabled", cfg.SecretKey != ""),
		Field("TLS Client Auth", cfg.TLSClientAuthMode()),
		Field("JWT Enabled", cfg.SecretKey != ""),
		Field("Request Body Limit", cfg.RequestBodyLimitBytes),
		Field("XRAY_LOCATION_ASSET", config.XrayAssetDir()),
	)
}

func ListenSummary(title string, addr string, protocol string) string {
	return Table(title,
		Field("Address", addr),
		Field("Protocol", protocol),
	)
}
