package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

type InternalController struct {
	state  *state.RuntimeState
	logger *slog.Logger
}

func (ctrl InternalController) GetConfig(c *gin.Context) {
	config := ctrl.state.Snapshot().CurrentConfig
	if config == nil {
		config = map[string]any{}
	}
	httpapi.WriteJSON(c, http.StatusOK, config)
}
