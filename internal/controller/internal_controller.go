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
	httpapi.WriteJSON(c, http.StatusOK, map[string]any{
		"xrayConfig": ctrl.state.Snapshot().CurrentConfig,
	})
}

func (ctrl InternalController) Webhook(c *gin.Context) {
	httpapi.WriteJSON(c, http.StatusOK, map[string]any{
		"accepted": true,
	})
}
