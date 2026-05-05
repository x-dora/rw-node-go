package controller

import (
	"log/slog"
	"net"
	"strconv"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/system"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type Registry struct {
	Xray     XrayController
	Handler  HandlerController
	Stats    StatsController
	Vision   VisionController
	Plugin   PluginController
	Internal InternalController
	Snapshot system.Snapshotter
}

func NewRegistry(cfg config.Config, runtimeState *state.RuntimeState, logger *slog.Logger) Registry {
	internalMTLS, err := xray.NewInternalMTLSBundle()
	if err != nil {
		panic(err)
	}
	apiAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.XTLSAPIPort))
	core := xray.NewProcessCore(
		cfg.XrayBin,
		cfg.XrayConfigPath,
		apiAddress,
		internalMTLS,
	)
	builder := xray.ConfigBuilder{XTLSAPIPort: cfg.XTLSAPIPort, InternalMTLS: internalMTLS}
	return NewRegistryWithXrayAndSnapshotter(runtimeState, logger, core, builder, system.NewSnapshotter())
}

func NewRegistryWithXray(runtimeState *state.RuntimeState, logger *slog.Logger, core xray.Core, builder xray.ConfigBuilder) Registry {
	return NewRegistryWithXrayAndSnapshotter(runtimeState, logger, core, builder, system.NewSnapshotter())
}

func NewRegistryWithXrayAndSnapshotter(runtimeState *state.RuntimeState, logger *slog.Logger, core xray.Core, builder xray.ConfigBuilder, snapshotter system.Snapshotter) Registry {
	return Registry{
		Xray:     XrayController{state: runtimeState, logger: logger, core: core, builder: builder, snapshot: snapshotter},
		Handler:  HandlerController{state: runtimeState, logger: logger, core: core},
		Stats:    StatsController{state: runtimeState, logger: logger, core: core, snapshot: snapshotter},
		Vision:   VisionController{state: runtimeState, logger: logger},
		Plugin:   PluginController{state: runtimeState, logger: logger},
		Internal: InternalController{state: runtimeState, logger: logger},
		Snapshot: snapshotter,
	}
}
