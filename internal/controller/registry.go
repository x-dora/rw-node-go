package controller

import (
	"log/slog"
	"net"
	"strconv"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type Registry struct {
	Xray     XrayController
	Handler  HandlerController
	Stats    StatsController
	Vision   VisionController
	Plugin   PluginController
	Internal InternalController
}

func NewRegistry(cfg config.Config, runtimeState *state.RuntimeState, logger *slog.Logger) Registry {
	core := xray.NewProcessCore(
		cfg.XrayBin,
		cfg.XrayConfigPath,
		net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.XTLSAPIPort)),
	)
	builder := xray.ConfigBuilder{XTLSAPIPort: cfg.XTLSAPIPort}
	return NewRegistryWithXray(runtimeState, logger, core, builder)
}

func NewRegistryWithXray(runtimeState *state.RuntimeState, logger *slog.Logger, core xray.Core, builder xray.ConfigBuilder) Registry {
	return Registry{
		Xray:     XrayController{state: runtimeState, logger: logger, core: core, builder: builder},
		Handler:  HandlerController{state: runtimeState, logger: logger},
		Stats:    StatsController{state: runtimeState, logger: logger},
		Vision:   VisionController{state: runtimeState, logger: logger},
		Plugin:   PluginController{state: runtimeState, logger: logger},
		Internal: InternalController{state: runtimeState, logger: logger},
	}
}
