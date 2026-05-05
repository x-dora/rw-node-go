package controller

import (
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
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
	var webhook contracts.XrayWebhookReport
	if err := c.ShouldBindJSON(&webhook); err != nil {
		ctrl.logger.Debug("ignore invalid xray webhook", "error", err)
		httpapi.WriteJSON(c, http.StatusOK, map[string]any{"accepted": true})
		return
	}
	if webhook.Network == "" || webhook.Destination == "" || webhook.Email == nil || *webhook.Email == "" {
		httpapi.WriteJSON(c, http.StatusOK, map[string]any{"accepted": true})
		return
	}
	ip := extractWebhookIP(webhook.Source)
	if ip == "" {
		httpapi.WriteJSON(c, http.StatusOK, map[string]any{"accepted": true})
		return
	}
	if ctrl.state.AddTorrentBlockerReport(webhook, ip, *webhook.Email, time.Now()) {
		ctrl.logger.Info("torrent blocker report collected", "ip", ip, "user", *webhook.Email)
	}
	httpapi.WriteJSON(c, http.StatusOK, map[string]any{
		"accepted": true,
	})
}

var webhookSourcePattern = regexp.MustCompile(`^(?:(?:tcp|udp):)?(?:\[(.+?)\]|(.+?))(?::(\d+))?$`)

func extractWebhookIP(source *string) string {
	if source == nil || *source == "" {
		return ""
	}
	candidate := *source
	if matches := webhookSourcePattern.FindStringSubmatch(candidate); len(matches) > 0 {
		if matches[1] != "" {
			candidate = matches[1]
		} else if matches[2] != "" {
			candidate = matches[2]
		}
	}
	if net.ParseIP(candidate) == nil {
		return ""
	}
	return candidate
}
