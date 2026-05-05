package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
)

type HandlerController struct {
	state  *state.RuntimeState
	logger *slog.Logger
}

func (ctrl HandlerController) AddUser(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SuccessResponse())
}

func (ctrl HandlerController) AddUsers(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SuccessResponse())
}

func (ctrl HandlerController) RemoveUser(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SuccessResponse())
}

func (ctrl HandlerController) RemoveUsers(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SuccessResponse())
}

func (ctrl HandlerController) GetInboundUsers(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersResponse{Users: []contracts.InboundUser{}})
}

func (ctrl HandlerController) GetInboundUsersCount(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersCountResponse{Count: 0})
}

func (ctrl HandlerController) DropUsersConnections(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SimpleSuccess())
}

func (ctrl HandlerController) DropIPs(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SimpleSuccess())
}
