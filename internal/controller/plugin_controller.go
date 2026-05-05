package controller

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type PluginController struct {
	state  *state.RuntimeState
	logger *slog.Logger
	core   xray.Core
}

func (ctrl PluginController) Sync(c *gin.Context) {
	var request contracts.PluginSyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: false})
		return
	}
	if request.Plugin == nil {
		accepted := ctrl.state.HasActivePlugin()
		if accepted {
			ctrl.state.ResetPlugins()
			ctrl.stopXrayForPluginChange(c)
		}
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: accepted})
		return
	}

	configChanged := ctrl.state.IsPluginConfigChanged(request.Plugin.Config)
	config, err := parseTorrentBlockerConfig(request.Plugin.Config)
	if err != nil {
		ctrl.logger.Warn("sync torrent blocker plugin config", "error", err)
		ctrl.state.ResetPlugins()
		ctrl.stopXrayForPluginChange(c)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: false})
		return
	}

	before := ctrl.state.TorrentBlockerSnapshot()
	ctrl.state.SyncTorrentBlockerPlugin(request.Plugin.UUID, request.Plugin.Name, request.Plugin.Config, config)
	after := ctrl.state.TorrentBlockerSnapshot()
	if configChanged && torrentBlockerRestartNeeded(before, after) {
		ctrl.stopXrayForPluginChange(c)
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.AcceptedResponse{Accepted: true})
}

func (ctrl PluginController) CollectTorrentBlockerReports(c *gin.Context) {
	reports := ctrl.state.FlushTorrentBlockerReports()
	if reports == nil {
		reports = []contracts.TorrentBlockerReport{}
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.TorrentBlockerReportsResponse{
		Reports: reports,
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

func parseTorrentBlockerConfig(config map[string]any) (state.TorrentBlockerConfig, error) {
	torrentValue, ok := config["torrentBlocker"]
	if !ok || torrentValue == nil {
		return state.TorrentBlockerConfig{}, nil
	}
	torrent, ok := torrentValue.(map[string]any)
	if !ok {
		return state.TorrentBlockerConfig{}, errors.New("torrentBlocker must be an object")
	}
	if enabled, ok := torrent["enabled"].(bool); !ok || !enabled {
		return state.TorrentBlockerConfig{}, nil
	}

	duration, ok := numberAsInt(torrent["blockDuration"])
	if !ok || duration < 0 {
		return state.TorrentBlockerConfig{}, errors.New("torrentBlocker.blockDuration must be a non-negative number")
	}

	ignoreLists, ok := torrent["ignoreLists"].(map[string]any)
	if !ok {
		return state.TorrentBlockerConfig{}, errors.New("torrentBlocker.ignoreLists must be an object")
	}

	sharedLists, err := parseSharedIPLists(config["sharedLists"])
	if err != nil {
		return state.TorrentBlockerConfig{}, err
	}

	ips, err := stringArray(ignoreLists["ip"])
	if err != nil {
		return state.TorrentBlockerConfig{}, errors.New("torrentBlocker.ignoreLists.ip must be a string array")
	}
	resolvedIPs := resolveSharedIPs(ips, sharedLists)

	users, err := stringArray(ignoreLists["userId"])
	if err != nil {
		return state.TorrentBlockerConfig{}, errors.New("torrentBlocker.ignoreLists.userId must be a string array")
	}

	includeRuleTags, err := stringArray(torrent["includeRuleTags"])
	if err != nil {
		return state.TorrentBlockerConfig{}, errors.New("torrentBlocker.includeRuleTags must be a string array")
	}

	return state.TorrentBlockerConfig{
		Enabled:         true,
		BlockDuration:   duration,
		IgnoredIPs:      resolvedIPs,
		IgnoredUsers:    users,
		IncludeRuleTags: includeRuleTags,
	}, nil
}

func parseSharedIPLists(value any) (map[string][]string, error) {
	lists := map[string][]string{}
	if value == nil {
		return lists, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("sharedLists must be an array")
	}
	for _, item := range items {
		list, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("sharedLists entries must be objects")
		}
		if list["type"] != "ipList" {
			continue
		}
		name, ok := list["name"].(string)
		if !ok || name == "" {
			return nil, errors.New("shared ip list name must be a string")
		}
		values, err := stringArray(list["items"])
		if err != nil {
			return nil, errors.New("shared ip list items must be a string array")
		}
		lists["ext:"+name] = values
	}
	return lists, nil
}

func resolveSharedIPs(ips []string, sharedLists map[string][]string) []string {
	output := make([]string, 0, len(ips))
	for _, ip := range ips {
		if strings.HasPrefix(ip, "ext:") {
			output = append(output, sharedLists[ip]...)
			continue
		}
		output = append(output, ip)
	}
	return output
}

func stringArray(value any) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("value must be an array")
	}
	output := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, errors.New("array item must be a string")
		}
		output = append(output, text)
	}
	return output, nil
}

func numberAsInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func torrentBlockerRestartNeeded(before, after state.TorrentBlockerSnapshot) bool {
	if before.Enabled != after.Enabled {
		return true
	}
	if !sameStringSet(before.IncludeRuleTags, after.IncludeRuleTags) {
		return true
	}
	return false
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, value := range a {
		counts[value]++
	}
	for _, value := range b {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}

func (ctrl PluginController) stopXrayForPluginChange(c *gin.Context) {
	if ctrl.core == nil || !ctrl.core.IsRunning() {
		ctrl.state.SetXrayRunning(false)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := ctrl.core.Stop(ctx); err != nil {
		ctrl.logger.Warn("stop xray after plugin sync", "error", err)
	}
	ctrl.state.SetXrayRunning(false)
}
