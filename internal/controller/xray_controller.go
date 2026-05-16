package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/logview"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/system"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type XrayController struct {
	state    *state.RuntimeState
	logger   *slog.Logger
	core     xray.Core
	builder  xray.ConfigBuilder
	snapshot system.Snapshotter
}

func (ctrl XrayController) Start(c *gin.Context) {
	startedAt := time.Now()
	masterIP := c.ClientIP()
	var request contracts.StartXrayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		errMsg := err.Error()
		ctrl.logStartFailure(masterIP, nil, err, time.Since(startedAt))
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.StartXrayResponse{
			IsStarted:       false,
			Version:         ctrl.state.Snapshot().XrayVersion,
			Error:           &errMsg,
			NodeInformation: contracts.NodeInformation{Version: ptr(ctrl.state.NodeVersion)},
			System:          ctrl.systemStats(c.Request.Context()),
		})
		return
	}

	hashes := state.HashesFromContract(request.Internals.Hashes)
	coreRunning := ctrl.core.IsRunning()
	ctrl.logStartRequest(masterIP, request.Internals.ForceRestart, coreRunning, hashes)
	decision := ctrl.state.RestartDecision(request.Internals.ForceRestart, hashes, coreRunning)
	ctrl.logRestartDecision(decision)
	if !decision.ShouldRestart {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		err := ctrl.core.Health(ctx)
		cancel()
		if err == nil {
			ctrl.state.SetXrayInternalStatusCached(true)
			ctrl.logger.Info("Xray Core configuration is up-to-date - no restart required", "duration", logview.Duration(time.Since(startedAt)))
			snapshot := ctrl.state.Snapshot()
			httpapi.WriteEnvelope(c, http.StatusOK, contracts.StartXrayResponse{
				IsStarted:       true,
				Version:         snapshot.XrayVersion,
				Error:           nil,
				NodeInformation: contracts.NodeInformation{Version: ptr(snapshot.NodeVersion)},
				System:          ctrl.systemStats(c.Request.Context()),
			})
			return
		}
		if err != nil {
			ctrl.logger.Warn("xray internal health check failed, restarting", "error", logview.RedactText(err.Error()))
			logview.WarnTable(ctrl.logger, "Xray health check failed - restart required", logview.Table("Xray restart required",
				logview.Field("Master IP", masterIP),
				logview.Field("Reason", "health_check_failed"),
				logview.Field("Error", logview.RedactText(err.Error())),
			))
			ctrl.state.SetXrayInternalStatusCached(false)
		}
	}

	fullConfig, err := ctrl.builder.Build(request.XrayConfig)
	if err != nil {
		ctrl.logStartFailure(masterIP, ctrl.state.Snapshot().XrayVersion, err, time.Since(startedAt))
		ctrl.writeStartError(c, err)
		return
	}
	ctrl.logConfigReceived(fullConfig)

	configBytes, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		ctrl.logStartFailure(masterIP, ctrl.state.Snapshot().XrayVersion, err, time.Since(startedAt))
		ctrl.writeStartError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if err := ctrl.core.Start(ctx, configBytes); err != nil {
		ctrl.state.SetXrayRunning(false)
		ctrl.logStartFailure(masterIP, ctrl.state.Snapshot().XrayVersion, err, time.Since(startedAt))
		ctrl.writeStartError(c, err)
		return
	}

	version, err := ctrl.core.Version(ctx)
	var versionPtr *string
	if err == nil && version != "" {
		versionPtr = &version
	}

	ctrl.state.SetXrayStarted(versionPtr, fullConfig, hashes)
	ctrl.state.SetInboundProtocolsFromConfig(fullConfig)
	ctrl.logStartSuccess(masterIP, versionPtr, decision, hashes, time.Since(startedAt))
	ctrl.logger.Info("Attempt to start XTLS took", "duration", logview.Duration(time.Since(startedAt)))
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.StartXrayResponse{
		IsStarted:       true,
		Version:         versionPtr,
		Error:           nil,
		NodeInformation: contracts.NodeInformation{Version: ptr(ctrl.state.NodeVersion)},
		System:          ctrl.systemStats(c.Request.Context()),
	})
}

func (ctrl XrayController) Stop(c *gin.Context) {
	startedAt := time.Now()
	snapshot := ctrl.state.Snapshot()
	ctrl.logger.Info("Remnawave requested to stop Xray")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := ctrl.core.Stop(ctx); err != nil {
		ctrl.logger.Warn("stop xray", "error", logview.RedactText(err.Error()))
		logview.WarnTable(ctrl.logger, "Xray stop returned error", logview.Table("Xray stop error",
			logview.Field("Was Running", snapshot.XrayRunning),
			logview.Field("Version", snapshot.XrayVersion),
			logview.Field("Error", logview.RedactText(err.Error())),
			logview.Field("Duration", time.Since(startedAt)),
		))
	}
	ctrl.state.SetXrayRunning(false)
	logview.InfoTable(ctrl.logger, "Xray stopped", logview.Table("Xray stopped",
		logview.Field("Was Running", snapshot.XrayRunning),
		logview.Field("Version", snapshot.XrayVersion),
		logview.Field("Duration", time.Since(startedAt)),
	))
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.StopXrayResponse{IsStopped: true})
}

func (ctrl XrayController) Healthcheck(c *gin.Context) {
	snapshot := ctrl.state.Snapshot()
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.HealthcheckResponse{
		IsAlive:                  true,
		XrayInternalStatusCached: snapshot.XrayInternalStatusCached,
		XrayVersion:              snapshot.XrayVersion,
		NodeVersion:              snapshot.NodeVersion,
	})
}

func (ctrl XrayController) writeStartError(c *gin.Context, err error) {
	errMsg := err.Error()
	ctrl.state.SetXrayInternalStatusCached(false)
	snapshot := ctrl.state.Snapshot()
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.StartXrayResponse{
		IsStarted:       false,
		Version:         snapshot.XrayVersion,
		Error:           &errMsg,
		NodeInformation: contracts.NodeInformation{Version: ptr(snapshot.NodeVersion)},
		System:          ctrl.systemStats(c.Request.Context()),
	})
}

func (ctrl XrayController) logStartRequest(masterIP string, forceRestart bool, coreRunning bool, hashes state.Hashes) {
	logview.InfoTable(ctrl.logger, "Xray start request", logview.Table("Xray start request",
		logview.Field("Master IP", masterIP),
		logview.Field("Force Restart", forceRestart),
		logview.Field("Core Running", coreRunning),
		logview.Field("Incoming Inbounds", len(hashes.Inbounds)),
		logview.Field("Incoming Users", totalInboundUsers(hashes)),
		logview.Field("Empty Config Hash", logview.ShortHash(hashes.EmptyConfig)),
	))
	logview.InfoTable(ctrl.logger, "Xray start inbound hashes", logview.InboundTable("Xray start inbound hashes", inboundRows(hashes)))
}

func (ctrl XrayController) logRestartDecision(decision state.RestartDecision) {
	switch decision.Reason {
	case state.RestartReasonForce:
		ctrl.logger.Warn("Force restart requested")
	case state.RestartReasonCoreNotRunning:
		ctrl.logger.Info("Xray core is not running - start required")
	case state.RestartReasonNoPreviousHashes:
		ctrl.logger.Info("Xray Core previous configuration hash is empty - start required")
	case state.RestartReasonEmptyConfigHashChange:
		ctrl.logger.Warn("Detected changes in Xray Core base configuration",
			"previous_hash", logview.ShortHash(decision.PreviousHash),
			"incoming_hash", logview.ShortHash(decision.IncomingHash),
		)
	case state.RestartReasonInboundCountChange:
		ctrl.logger.Warn("Number of Xray Core inbounds has changed")
	case state.RestartReasonInboundRemoved:
		ctrl.logger.Warn("Inbound no longer exists in Xray Core configuration",
			"tag", decision.InboundTag,
			"previous_hash", logview.ShortHash(decision.PreviousHash),
		)
	case state.RestartReasonInboundHashChange:
		ctrl.logger.Warn("User configuration changed for inbound",
			"tag", decision.InboundTag,
			"previous_hash", logview.ShortHash(decision.PreviousHash),
			"incoming_hash", logview.ShortHash(decision.IncomingHash),
		)
	}
}

func (ctrl XrayController) logConfigReceived(config map[string]any) {
	logview.InfoTable(ctrl.logger, "Xray config received", logview.Table("Xray config received",
		logview.Field("Inbounds", arrayLen(config["inbounds"])),
		logview.Field("Outbounds", arrayLen(config["outbounds"])),
		logview.Field("Routing Rules", routingRulesLen(config)),
		logview.Field("Stats Enabled", hasMap(config["stats"])),
		logview.Field("Policy Enabled", hasMap(config["policy"])),
	))
}

func (ctrl XrayController) logStartSuccess(masterIP string, version *string, decision state.RestartDecision, hashes state.Hashes, duration time.Duration) {
	action := "Started"
	if decision.Reason != state.RestartReasonCoreNotRunning && decision.Reason != state.RestartReasonNoPreviousHashes {
		action = "Restarted"
	}
	logview.InfoTable(ctrl.logger, "Xray started", logview.Table("Xray started",
		logview.Field("Version", version),
		logview.Field("Master IP", masterIP),
		logview.Field("Action", action),
		logview.Field("Internal Status", true),
		logview.Field("Inbounds", len(hashes.Inbounds)),
		logview.Field("Users", totalInboundUsers(hashes)),
		logview.Field("Duration", duration),
	))
}

func (ctrl XrayController) logStartFailure(masterIP string, previousVersion *string, err error, duration time.Duration) {
	if err == nil {
		return
	}
	logview.ErrorTable(ctrl.logger, "Xray failed to start", logview.Table("Xray failed to start",
		logview.Field("Previous Version", previousVersion),
		logview.Field("Master IP", masterIP),
		logview.Field("Internal Status", false),
		logview.Field("Error", logview.RedactText(err.Error())),
		logview.Field("Duration", duration),
		logview.Field("Diagnostics", "previous config/hash/version preserved"),
	))
	ctrl.logger.Info("Attempt to start XTLS took", "duration", logview.Duration(duration))
}

func ptr(value string) *string {
	return &value
}

func (ctrl XrayController) systemStats(ctx context.Context) contracts.SystemStatsPayload {
	if ctrl.snapshot == nil {
		return system.SnapshotStats()
	}
	return ctrl.snapshot.SnapshotStats(ctx)
}

func totalInboundUsers(hashes state.Hashes) int {
	total := 0
	for _, inbound := range hashes.Inbounds {
		total += inbound.UsersCount
	}
	return total
}

func inboundRows(hashes state.Hashes) []logview.InboundRow {
	rows := make([]logview.InboundRow, 0, len(hashes.Inbounds))
	for _, inbound := range hashes.Inbounds {
		rows = append(rows, logview.InboundRow{
			Tag:        inbound.Tag,
			UsersCount: inbound.UsersCount,
			Hash:       inbound.Hash,
		})
	}
	return rows
}

func arrayLen(value any) int {
	items, ok := value.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func hasMap(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}

func routingRulesLen(config map[string]any) int {
	routing, ok := config["routing"].(map[string]any)
	if !ok {
		return 0
	}
	return arrayLen(routing["rules"])
}
