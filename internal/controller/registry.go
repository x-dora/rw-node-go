package controller

import (
	"log/slog"

	"github.com/x-dora/rw-node-go/internal/state"
)

type Registry struct {
	Xray     XrayController
	Handler  HandlerController
	Stats    StatsController
	Vision   VisionController
	Plugin   PluginController
	Internal InternalController
}

func NewRegistry(runtimeState *state.RuntimeState, logger *slog.Logger) Registry {
	return Registry{
		Xray:     XrayController{state: runtimeState, logger: logger},
		Handler:  HandlerController{state: runtimeState, logger: logger},
		Stats:    StatsController{state: runtimeState, logger: logger},
		Vision:   VisionController{state: runtimeState, logger: logger},
		Plugin:   PluginController{state: runtimeState, logger: logger},
		Internal: InternalController{state: runtimeState, logger: logger},
	}
}
