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
	var request contracts.StartXrayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		errMsg := err.Error()
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
	shouldRestart := ctrl.state.ShouldRestart(request.Internals.ForceRestart, hashes, ctrl.core.IsRunning())
	if !shouldRestart {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		err := ctrl.core.Health(ctx)
		cancel()
		if err != nil {
			ctrl.logger.Warn("xray internal health check failed, restarting", "error", err)
			ctrl.state.SetXrayInternalStatusCached(false)
			shouldRestart = true
		} else {
			ctrl.state.SetXrayInternalStatusCached(true)
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
	}

	if !shouldRestart {
		return
	}

	fullConfig, err := ctrl.builder.Build(request.XrayConfig)
	if err != nil {
		ctrl.writeStartError(c, err)
		return
	}

	configBytes, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		ctrl.writeStartError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if err := ctrl.core.Start(ctx, configBytes); err != nil {
		ctrl.state.SetXrayRunning(false)
		ctrl.writeStartError(c, err)
		return
	}

	version, err := ctrl.core.Version(ctx)
	var versionPtr *string
	if err == nil && version != "" {
		versionPtr = &version
	}

	ctrl.state.SetXrayStarted(versionPtr, fullConfig, hashes)
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.StartXrayResponse{
		IsStarted:       true,
		Version:         versionPtr,
		Error:           nil,
		NodeInformation: contracts.NodeInformation{Version: ptr(ctrl.state.NodeVersion)},
		System:          ctrl.systemStats(c.Request.Context()),
	})
}

func (ctrl XrayController) Stop(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := ctrl.core.Stop(ctx); err != nil {
		ctrl.logger.Warn("stop xray", "error", err)
	}
	ctrl.state.SetXrayRunning(false)
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

func ptr(value string) *string {
	return &value
}

func (ctrl XrayController) systemStats(ctx context.Context) contracts.SystemStatsPayload {
	if ctrl.snapshot == nil {
		return system.SnapshotStats()
	}
	return ctrl.snapshot.SnapshotStats(ctx)
}
