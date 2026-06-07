package controller

import (
	"log/slog"

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
		Stats:    StatsController{state: runtimeState, logger: logger, core: core, snapshot: snapshotter},
		Plugin:   PluginController{state: runtimeState, logger: logger, core: core},
		Internal: InternalController{state: runtimeState, logger: logger},
		Snapshot: snapshotter,
	}
}
