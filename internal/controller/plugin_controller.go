package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

type PluginController struct {
	state  *state.RuntimeState
	logger *slog.Logger
}

func (ctrl PluginController) Sync(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: true})
}

func (ctrl PluginController) CollectTorrentBlockerReports(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.TorrentBlockerReportsResponse{
		Reports: []contracts.TorrentBlockerReport{},
	})
}

func (ctrl PluginController) BlockIPs(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: true})
}

func (ctrl PluginController) UnblockIPs(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: true})
}

func (ctrl PluginController) RecreateTables(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: true})
}
