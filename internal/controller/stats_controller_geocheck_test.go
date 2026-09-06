package controller

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/geocheck"
	"github.com/x-dora/rw-node-go/internal/state"
)

type fakeGeocheckRunner struct {
	report json.RawMessage
	err    error
}

func (f *fakeGeocheckRunner) Run(ctx context.Context, request geocheck.Request) (json.RawMessage, error) {
	return f.report, f.err
}

func TestStatsControllerGeocheckReturnsReportEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := StatsController{
		state:  state.NewRuntimeState(),
		logger: slog.Default(),
		geocheck: &fakeGeocheckRunner{report: json.RawMessage(
			`{"ip":"203.0.113.1","image":{"format":"svg","media_type":"image/svg+xml","encoding":"base64","data":"abc"}}`,
		)},
	}

	rec := runStatsRequest(t, ctrl.GetGeocheck, http.MethodPost, `{"ip":"203.0.113.1"}`)
	var body struct {
		Response struct {
			IP    string `json:"ip"`
			Image struct {
				Format    string `json:"format"`
				MediaType string `json:"media_type"`
				Encoding  string `json:"encoding"`
				Data      string `json:"data"`
			} `json:"image"`
		} `json:"response"`
	}
	decodeResponse(t, rec, &body)
	if body.Response.IP != "203.0.113.1" || body.Response.Image.Data != "abc" || body.Response.Image.Format != "svg" {
		t.Fatalf("geocheck body = %s", rec.Body.String())
	}
}

func TestStatsControllerGeocheckErrorsReturnA018(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := StatsController{
		state:    state.NewRuntimeState(),
		logger:   slog.Default(),
		geocheck: &fakeGeocheckRunner{err: errors.New("boom")},
	}

	failed := runStatsRequestStatus(t, ctrl.GetGeocheck, http.MethodPost, `{}`, http.StatusInternalServerError)
	assertOfficialError(t, failed, "A018", "boom")

	bind := runStatsRequestStatus(t, ctrl.GetGeocheck, http.MethodPost, `{`, http.StatusInternalServerError)
	assertOfficialError(t, bind, "A018", "Failed to get geocheck report")
}

func TestStatsControllerGeocheckInvalidIPReturnsA018(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	// The real runner validates the IP before exec, so no geocheck binary is needed.
	ctrl := StatsController{
		state:    state.NewRuntimeState(),
		logger:   slog.Default(),
		geocheck: geocheck.NewRunner(slog.Default()),
	}

	rec := runStatsRequestStatus(t, ctrl.GetGeocheck, http.MethodPost, `{"ip":"not-an-ip"}`, http.StatusInternalServerError)
	assertOfficialError(t, rec, "A018", `geocheck: "not-an-ip" is not a valid IP address`)
}

func TestStatsControllerGeocheckWithoutRunnerReturnsA018(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctrl := StatsController{state: state.NewRuntimeState(), logger: slog.Default()}

	rec := runStatsRequestStatus(t, ctrl.GetGeocheck, http.MethodPost, `{}`, http.StatusInternalServerError)
	assertOfficialError(t, rec, "A018", "Failed to get geocheck report")
}
