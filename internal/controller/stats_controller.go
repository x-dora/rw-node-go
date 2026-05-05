package controller

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/system"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type StatsController struct {
	state  *state.RuntimeState
	logger *slog.Logger
	core   xray.Core
}

func (ctrl StatsController) GetSystemStats(c *gin.Context) {
	response := contracts.SystemStatsResponse{
		XrayInfo: nil,
		Plugins:  emptyPluginStats(),
		System:   contracts.SystemStats{Stats: emptySystemStats().Stats},
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, response)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	sysStats, err := client.SysStats(ctx)
	if err != nil {
		ctrl.logger.Warn("get xray system stats", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, response)
		return
	}
	response.XrayInfo = contractXraySysStats(sysStats)
	httpapi.WriteEnvelope(c, http.StatusOK, response)
}

func (ctrl StatsController) GetUsersStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersStatsResponse{Users: []contracts.UserTrafficStats{}})
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersStatsResponse{Users: []contracts.UserTrafficStats{}})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	users, err := client.UsersStats(ctx, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get xray users stats", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersStatsResponse{Users: []contracts.UserTrafficStats{}})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersStatsResponse{Users: contractUserTrafficStats(users)})
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
	var request contracts.TaggedStatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundTrafficStatsResponse{})
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundTrafficStatsResponse{Inbound: request.Tag})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	stats, err := client.InboundStats(ctx, request.Tag, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get xray inbound stats", "tag", request.Tag, "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundTrafficStatsResponse{Inbound: request.Tag})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contractInboundTrafficStats(stats))
}

func (ctrl StatsController) GetOutboundStats(c *gin.Context) {
	var request contracts.TaggedStatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.OutboundTrafficStatsResponse{})
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.OutboundTrafficStatsResponse{Outbound: request.Tag})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	stats, err := client.OutboundStats(ctx, request.Tag, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get xray outbound stats", "tag", request.Tag, "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.OutboundTrafficStatsResponse{Outbound: request.Tag})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contractOutboundTrafficStats(stats))
}

func (ctrl StatsController) GetAllInboundsStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllInboundsStatsResponse{
			Inbounds: []contracts.InboundTrafficStatsResponse{},
		})
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllInboundsStatsResponse{
			Inbounds: []contracts.InboundTrafficStatsResponse{},
		})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	inbounds, err := client.AllInboundStats(ctx, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get all xray inbound stats", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllInboundsStatsResponse{
			Inbounds: []contracts.InboundTrafficStatsResponse{},
		})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllInboundsStatsResponse{
		Inbounds: contractInboundTrafficStatsList(inbounds),
	})
}

func (ctrl StatsController) GetAllOutboundsStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllOutboundsStatsResponse{
			Outbounds: []contracts.OutboundTrafficStatsResponse{},
		})
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllOutboundsStatsResponse{
			Outbounds: []contracts.OutboundTrafficStatsResponse{},
		})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	outbounds, err := client.AllOutboundStats(ctx, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get all xray outbound stats", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllOutboundsStatsResponse{
			Outbounds: []contracts.OutboundTrafficStatsResponse{},
		})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllOutboundsStatsResponse{
		Outbounds: contractOutboundTrafficStatsList(outbounds),
	})
}

func (ctrl StatsController) GetCombinedStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, emptyCombinedStatsResponse())
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, emptyCombinedStatsResponse())
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	inbounds, inboundErr := client.AllInboundStats(ctx, request.Reset)
	outbounds, outboundErr := client.AllOutboundStats(ctx, request.Reset)
	if inboundErr != nil || outboundErr != nil {
		ctrl.logger.Warn("get combined xray stats", "inboundError", inboundErr, "outboundError", outboundErr)
		httpapi.WriteEnvelope(c, http.StatusOK, emptyCombinedStatsResponse())
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.CombinedStatsResponse{
		Inbounds:  contractInboundTrafficStatsList(inbounds),
		Outbounds: contractOutboundTrafficStatsList(outbounds),
	})
}

func emptySystemStats() contracts.SystemStatsPayload {
	return system.SnapshotStats()
}

func emptyPluginStats() contracts.PluginStats {
	return contracts.PluginStats{
		TorrentBlocker: contracts.TorrentBlockerPluginStats{ReportsCount: 0},
	}
}

func emptyCombinedStatsResponse() contracts.CombinedStatsResponse {
	return contracts.CombinedStatsResponse{
		Inbounds:  []contracts.InboundTrafficStatsResponse{},
		Outbounds: []contracts.OutboundTrafficStatsResponse{},
	}
}

func (ctrl StatsController) statsClient() (xray.StatsClient, error) {
	if ctrl.core == nil || !ctrl.core.IsRunning() {
		return nil, errors.New("xray core not running")
	}
	client := ctrl.core.Stats()
	if client == nil {
		return nil, errors.New("xray stats client is unavailable")
	}
	return client, nil
}

func statsContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 10*time.Second)
}

func contractXraySysStats(stats xray.SysStats) *contracts.XraySysStats {
	return &contracts.XraySysStats{
		NumGoroutine: stats.NumGoroutine,
		NumGC:        stats.NumGC,
		Alloc:        stats.Alloc,
		TotalAlloc:   stats.TotalAlloc,
		Sys:          stats.Sys,
		Mallocs:      stats.Mallocs,
		Frees:        stats.Frees,
		LiveObjects:  stats.LiveObjects,
		PauseTotalNs: stats.PauseTotalNs,
		Uptime:       stats.Uptime,
	}
}

func contractUserTrafficStats(users []xray.UserTrafficStats) []contracts.UserTrafficStats {
	output := make([]contracts.UserTrafficStats, 0, len(users))
	for _, user := range users {
		output = append(output, contracts.UserTrafficStats{
			Username: user.Username,
			Downlink: user.Downlink,
			Uplink:   user.Uplink,
		})
	}
	return output
}

func contractInboundTrafficStats(stats xray.InboundTrafficStats) contracts.InboundTrafficStatsResponse {
	return contracts.InboundTrafficStatsResponse{
		Inbound:  stats.Inbound,
		Downlink: stats.Downlink,
		Uplink:   stats.Uplink,
	}
}

func contractOutboundTrafficStats(stats xray.OutboundTrafficStats) contracts.OutboundTrafficStatsResponse {
	return contracts.OutboundTrafficStatsResponse{
		Outbound: stats.Outbound,
		Downlink: stats.Downlink,
		Uplink:   stats.Uplink,
	}
}

func contractInboundTrafficStatsList(stats []xray.InboundTrafficStats) []contracts.InboundTrafficStatsResponse {
	output := make([]contracts.InboundTrafficStatsResponse, 0, len(stats))
	for _, item := range stats {
		output = append(output, contractInboundTrafficStats(item))
	}
	return output
}

func contractOutboundTrafficStatsList(stats []xray.OutboundTrafficStats) []contracts.OutboundTrafficStatsResponse {
	output := make([]contracts.OutboundTrafficStatsResponse, 0, len(stats))
	for _, item := range stats {
		output = append(output, contractOutboundTrafficStats(item))
	}
	return output
}
