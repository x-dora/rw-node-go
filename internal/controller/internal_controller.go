package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

type InternalController struct {
	state    *state.RuntimeState
	logger   *slog.Logger
	panelSni string
}

func (ctrl InternalController) GetConfig(c *gin.Context) {
	config := ctrl.state.Snapshot().CurrentConfig
	if config == nil {
		config = map[string]any{}
	}
	// The derived SNI is tooling metadata for local consumers (the inbound
	// watcher front proxy); it is not part of the Panel-provided Xray config.
	// Only a copy of the snapshot is mutated, never the stored state.
	if ctrl.panelSni != "" {
		sniConfig := make(map[string]any, len(config)+1)
		for key, value := range config {
			sniConfig[key] = value
		}
		sniConfig["panelSni"] = ctrl.panelSni
		config = sniConfig
	}
	httpapi.WriteJSON(c, http.StatusOK, config)
}
