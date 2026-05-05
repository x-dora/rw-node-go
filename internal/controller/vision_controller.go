package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

type VisionController struct {
	state  *state.RuntimeState
	logger *slog.Logger
}

func (ctrl VisionController) BlockIP(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SuccessResponse())
}

func (ctrl VisionController) UnblockIP(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SuccessResponse())
}
