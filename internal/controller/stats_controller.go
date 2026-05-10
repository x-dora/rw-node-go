package controller

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/system"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type StatsController struct {
	state    *state.RuntimeState
	logger   *slog.Logger
	core     xray.Core
	snapshot system.Snapshotter
}

func (ctrl StatsController) GetSystemStats(c *gin.Context) {
	systemStats := ctrl.systemStats(c.Request.Context())
	response := contracts.SystemStatsResponse{
		XrayInfo: nil,
		Plugins: contracts.PluginStats{
			TorrentBlocker: contracts.TorrentBlockerPluginStats{ReportsCount: ctrl.state.TorrentBlockerReportsCount()},
		},
		System: contracts.SystemStats{Stats: systemStats.Stats},
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		writeOfficialStatsError(c, "Failed to get system stats", contracts.ErrFailedToGetSystemStats)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	sysStats, err := client.SysStats(ctx)
	if err != nil {
		ctrl.logger.Warn("get xray system stats", "error", err)
		writeOfficialStatsError(c, "Failed to get system stats", contracts.ErrFailedToGetSystemStats)
		return
	}
	response.XrayInfo = contractXraySysStats(sysStats)
	httpapi.WriteEnvelope(c, http.StatusOK, response)
}

func (ctrl StatsController) GetUsersStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeOfficialStatsError(c, "Failed to get users stats", contracts.ErrFailedToGetUsersStats)
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		writeOfficialStatsError(c, "Failed to get users stats", contracts.ErrFailedToGetUsersStats)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	users, err := client.UsersStats(ctx, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get xray users stats", "error", err)
		writeOfficialStatsError(c, "Failed to get users stats", contracts.ErrFailedToGetUsersStats)
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersStatsResponse{Users: contractUserTrafficStats(users)})
}

func (ctrl StatsController) GetUserOnlineStatus(c *gin.Context) {
	var request contracts.UserOnlineStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserOnlineStatusResponse{IsOnline: false})
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserOnlineStatusResponse{IsOnline: false})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	online, err := client.UserOnlineStatus(ctx, request.Username)
	if err != nil {
		ctrl.logger.Warn("get xray user online status", "username", request.Username, "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserOnlineStatusResponse{IsOnline: false})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserOnlineStatusResponse{IsOnline: online})
}

func (ctrl StatsController) GetUserIPList(c *gin.Context) {
	var request contracts.UserIPListRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserIPListResponse{IPs: []contracts.IPLastSeen{}})
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserIPListResponse{IPs: []contracts.IPLastSeen{}})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	ips, err := client.UserIPList(ctx, request.UserID, true)
	if err != nil {
		ctrl.logger.Warn("get xray user IP list", "userId", request.UserID, "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserIPListResponse{IPs: []contracts.IPLastSeen{}})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UserIPListResponse{IPs: contractIPLastSeenList(ips)})
}

func (ctrl StatsController) GetUsersIPList(c *gin.Context) {
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersIPListResponse{Users: []contracts.UserIPList{}})
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	users, err := client.UsersIPList(ctx, true)
	if err != nil {
		ctrl.logger.Warn("get xray users IP list", "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersIPListResponse{Users: []contracts.UserIPList{}})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.UsersIPListResponse{Users: contractUsersIPList(users)})
}

func (ctrl StatsController) GetInboundStats(c *gin.Context) {
	var request contracts.TaggedStatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeOfficialStatsError(c, "Failed to get inbound stats", contracts.ErrFailedToGetInboundStats)
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		writeOfficialStatsError(c, "Failed to get inbound stats", contracts.ErrFailedToGetInboundStats)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	stats, err := client.InboundStats(ctx, request.Tag, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get xray inbound stats", "tag", request.Tag, "error", err)
		writeOfficialStatsError(c, "Failed to get inbound stats", contracts.ErrFailedToGetInboundStats)
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contractInboundTrafficStats(stats))
}

func (ctrl StatsController) GetOutboundStats(c *gin.Context) {
	var request contracts.TaggedStatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeOfficialStatsError(c, "Failed to get outbound stats", contracts.ErrFailedToGetOutboundStats)
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		writeOfficialStatsError(c, "Failed to get outbound stats", contracts.ErrFailedToGetOutboundStats)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	stats, err := client.OutboundStats(ctx, request.Tag, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get xray outbound stats", "tag", request.Tag, "error", err)
		writeOfficialStatsError(c, "Failed to get outbound stats", contracts.ErrFailedToGetOutboundStats)
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contractOutboundTrafficStats(stats))
}

func (ctrl StatsController) GetAllInboundsStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeOfficialStatsError(c, "Failed to get inbounds stats", contracts.ErrFailedToGetInboundsStats)
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		writeOfficialStatsError(c, "Failed to get inbounds stats", contracts.ErrFailedToGetInboundsStats)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	inbounds, err := client.AllInboundStats(ctx, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get all xray inbound stats", "error", err)
		writeOfficialStatsError(c, "Failed to get inbounds stats", contracts.ErrFailedToGetInboundsStats)
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllInboundsStatsResponse{
		Inbounds: contractInboundTrafficStatsList(inbounds),
	})
}

func (ctrl StatsController) GetAllOutboundsStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeOfficialStatsError(c, "Failed to get outbounds stats", contracts.ErrFailedToGetOutboundsStats)
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		writeOfficialStatsError(c, "Failed to get outbounds stats", contracts.ErrFailedToGetOutboundsStats)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	outbounds, err := client.AllOutboundStats(ctx, request.Reset)
	if err != nil {
		ctrl.logger.Warn("get all xray outbound stats", "error", err)
		writeOfficialStatsError(c, "Failed to get outbounds stats", contracts.ErrFailedToGetOutboundsStats)
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AllOutboundsStatsResponse{
		Outbounds: contractOutboundTrafficStatsList(outbounds),
	})
}

func (ctrl StatsController) GetCombinedStats(c *gin.Context) {
	var request contracts.ResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeOfficialStatsError(c, "Failed to get combined stats", contracts.ErrFailedToGetCombinedStats)
		return
	}
	client, err := ctrl.statsClient()
	if err != nil {
		ctrl.logger.Debug("xray stats client unavailable", "error", err)
		writeOfficialStatsError(c, "Failed to get combined stats", contracts.ErrFailedToGetCombinedStats)
		return
	}

	ctx, cancel := statsContext(c)
	defer cancel()
	inbounds, inboundErr := client.AllInboundStats(ctx, request.Reset)
	outbounds, outboundErr := client.AllOutboundStats(ctx, request.Reset)
	if inboundErr != nil || outboundErr != nil {
		ctrl.logger.Warn("get combined xray stats", "inboundError", inboundErr, "outboundError", outboundErr)
		writeOfficialStatsError(c, "Failed to get combined stats", contracts.ErrFailedToGetCombinedStats)
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.CombinedStatsResponse{
		Inbounds:  contractInboundTrafficStatsList(inbounds),
		Outbounds: contractOutboundTrafficStatsList(outbounds),
	})
}

func (ctrl StatsController) systemStats(ctx context.Context) contracts.SystemStatsPayload {
	if ctrl.snapshot == nil {
		return system.SnapshotStats()
	}
	return ctrl.snapshot.SnapshotStats(ctx)
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

func writeOfficialStatsError(c *gin.Context, message string, code string) {
	httpapi.WriteOfficialError(c, http.StatusInternalServerError, message, code)
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
		if user.Uplink == 0 && user.Downlink == 0 {
			continue
		}
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

func contractIPLastSeenList(ips []xray.IPLastSeen) []contracts.IPLastSeen {
	output := make([]contracts.IPLastSeen, 0, len(ips))
	for _, item := range ips {
		output = append(output, contracts.IPLastSeen{
			IP:       item.IP,
			LastSeen: time.Unix(item.LastSeen, 0).UTC().Format(time.RFC3339),
		})
	}
	return output
}

func contractUsersIPList(users []xray.UserIPList) []contracts.UserIPList {
	output := make([]contracts.UserIPList, 0, len(users))
	for _, user := range users {
		output = append(output, contracts.UserIPList{
			UserID: user.Username,
			IPs:    contractIPLastSeenList(user.IPs),
		})
	}
	sort.Slice(output, func(i, j int) bool {
		return output[i].UserID < output[j].UserID
	})
	return output
}
