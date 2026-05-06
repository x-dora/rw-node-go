package controller

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type VisionController struct {
	state  *state.RuntimeState
	logger *slog.Logger
	core   xray.Core
}

func (ctrl VisionController) BlockIP(c *gin.Context) {
	request, ok := bindVisionIP(c)
	if !ok {
		return
	}
	ctrl.updateSourceIPRule(c, request.IP, true)
}

func (ctrl VisionController) UnblockIP(c *gin.Context) {
	request, ok := bindVisionIP(c)
	if !ok {
		return
	}
	ctrl.updateSourceIPRule(c, request.IP, false)
}

func (ctrl VisionController) updateSourceIPRule(c *gin.Context, ip string, block bool) {
	routing := ctrl.routingClient()
	if routing == nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.ErrorResponse("xray routing client is unavailable"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	ruleTag := visionRuleTag(ip)
	var err error
	if block {
		err = routing.AddSourceIPRule(ctx, ruleTag, ip, xray.BlockOutboundTag)
	} else {
		err = routing.RemoveRule(ctx, ruleTag)
	}
	if err != nil {
		ctrl.logger.Warn("update vision routing rule", "ip", ip, "block", block, "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.ErrorResponse(err.Error()))
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SuccessResponse())
}

func (ctrl VisionController) routingClient() xray.RoutingClient {
	if ctrl.core == nil || !ctrl.core.IsRunning() {
		return nil
	}
	return ctrl.core.Routing()
}

func bindVisionIP(c *gin.Context) (contracts.VisionIPRequest, bool) {
	var request contracts.VisionIPRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.ErrorResponse(err.Error()))
		return contracts.VisionIPRequest{}, false
	}
	return request, true
}

func visionRuleTag(ip string) string {
	data := fmt.Sprintf("string:%d:%s", len(ip), ip)
	sum := md5.Sum([]byte(data))
	return hex.EncodeToString(sum[:])
}
