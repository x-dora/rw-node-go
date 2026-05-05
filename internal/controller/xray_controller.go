package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

type XrayController struct {
	state  *state.RuntimeState
	logger *slog.Logger
}

func (ctrl XrayController) Start(c *gin.Context) {
	errMsg := "not implemented"
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.StartXrayResponse{
		IsStarted:       false,
		Version:         ctrl.state.XrayVersion,
		Error:           &errMsg,
		NodeInformation: contracts.NodeInformation{Version: ctrl.state.NodeVersion},
		System:          emptySystemStats(),
	})
}

func (ctrl XrayController) Stop(c *gin.Context) {
	ctrl.state.SetXrayRunning(false)
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.StopXrayResponse{IsStopped: true})
}

func (ctrl XrayController) Healthcheck(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.HealthcheckResponse{
		IsAlive:                  ctrl.state.IsXrayRunning(),
		XrayInternalStatusCached: false,
		XrayVersion:              ctrl.state.XrayVersion,
		NodeVersion:              ctrl.state.NodeVersion,
	})
}
