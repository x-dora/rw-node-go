package controller

import (
	"log/slog"
	"os"
	"strings"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/geocheck"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/system"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type Registry struct {
	Xray     *XrayController
	Handler  HandlerController
	Stats    StatsController
	Plugin   PluginController
	Internal InternalController
	Snapshot system.Snapshotter
}

func NewRegistry(runtimeState *state.RuntimeState, logger *slog.Logger) Registry {
	core := xray.NewEmbeddedCore()
	builder := xray.ConfigBuilder{StatsUserOnline: system.HasNetAdmin()}
	return NewRegistryWithXrayAndSnapshotter(runtimeState, logger, core, builder, system.NewSnapshotter())
}

func NewRegistryWithXray(runtimeState *state.RuntimeState, logger *slog.Logger, core xray.Core, builder xray.ConfigBuilder) Registry {
	return NewRegistryWithXrayAndSnapshotter(runtimeState, logger, core, builder, system.NewSnapshotter())
}

func NewRegistryWithXrayAndSnapshotter(runtimeState *state.RuntimeState, logger *slog.Logger, core xray.Core, builder xray.ConfigBuilder, snapshotter system.Snapshotter) Registry {
	return Registry{
		Xray:     &XrayController{state: runtimeState, logger: logger, core: core, builder: builder, snapshot: snapshotter},
		Handler:  HandlerController{state: runtimeState, logger: logger, core: core, dropper: system.Conntrack{}},
		Stats:    StatsController{state: runtimeState, logger: logger, core: core, snapshot: snapshotter, geocheck: geocheck.NewRunner(logger)},
		Plugin:   PluginController{state: runtimeState, logger: logger, core: core},
		Internal: InternalController{state: runtimeState, logger: logger, panelSni: derivePanelSni(logger)},
		Snapshot: snapshotter,
	}
}

// derivePanelSni resolves the SNI derived from the SECRET_KEY payload, when
// present, so /internal/get-config can expose it to the local inbound watcher
// front proxy. Derivation failures leave it empty (the watcher front is
// optional tooling and must not gate startup).
func derivePanelSni(logger *slog.Logger) string {
	secretKey := strings.TrimSpace(os.Getenv("SECRET_KEY"))
	if secretKey == "" {
		return ""
	}
	payload, err := config.DecodeSecretKey(secretKey)
	if err != nil {
		logger.Warn("decode SECRET_KEY for panel sni", "error", err)
		return ""
	}
	sni, err := httpapi.DeriveSNI(payload)
	if err != nil {
		logger.Warn("derive panel sni", "error", err)
		return ""
	}
	return sni
}
