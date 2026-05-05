package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

type StatsController struct {
	state  *state.RuntimeState
	logger *slog.Logger
}

func (ctrl StatsController) GetSystemStats(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SystemStatsResponse{System: emptySystemStats()})
}

func (ctrl StatsController) GetUsersStats(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersStatsResponse{Users: []contracts.UserTrafficStats{}})
}

func (ctrl StatsController) GetUserOnlineStatus(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserOnlineStatusResponse{IsOnline: false})
}

func (ctrl StatsController) GetUserIPList(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserIPListResponse{IPs: []contracts.IPLastSeen{}})
}

func (ctrl StatsController) GetUsersIPList(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersIPListResponse{Users: []contracts.UserIPList{}})
}

func (ctrl StatsController) GetInboundStats(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.TrafficStatsResponse{})
}

func (ctrl StatsController) GetOutboundStats(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.TrafficStatsResponse{})
}

func (ctrl StatsController) GetAllInboundsStats(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllInboundsStatsResponse{
		Inbounds: map[string]contracts.TrafficStatsResponse{},
	})
}

func (ctrl StatsController) GetAllOutboundsStats(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllOutboundsStatsResponse{
		Outbounds: map[string]contracts.TrafficStatsResponse{},
	})
}

func (ctrl StatsController) GetCombinedStats(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.CombinedStatsResponse{
		Users:     []contracts.UserTrafficStats{},
		Inbounds:  map[string]contracts.TrafficStatsResponse{},
		Outbounds: map[string]contracts.TrafficStatsResponse{},
		System:    emptySystemStats(),
	})
}

func emptySystemStats() contracts.SystemStatsPayload {
	return contracts.SystemStatsPayload{
		Info:      map[string]any{},
		Stats:     map[string]any{},
		Interface: map[string]any{},
	}
}
