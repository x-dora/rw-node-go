package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/contracts"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/xray"
)

type HandlerController struct {
	state  *state.RuntimeState
	logger *slog.Logger
	core   xray.Core
}

func (ctrl HandlerController) AddUser(c *gin.Context) {
	var request contracts.AddUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}
	if len(request.Data) == 0 {
		ctrl.writeSuccess(c, true, nil)
		return
	}

	for _, item := range request.Data {
		ctrl.state.AddKnownInboundTag(item.Tag)
	}

	client, err := ctrl.handlerClient()
	if err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}

	ctx, cancel := handlerContext(c)
	defer cancel()
	username := request.Data[0].Username
	removeHash := request.HashData.VlessUUID
	if request.HashData.PrevVlessUUID != nil && *request.HashData.PrevVlessUUID != "" {
		removeHash = *request.HashData.PrevVlessUUID
	}
	for _, tag := range ctrl.state.KnownInboundTags() {
		ctrl.removeUser(ctx, client, tag, username)
		ctrl.state.RemoveUserFromInbound(tag, removeHash)
	}

	success, firstErr := false, error(nil)
	for _, item := range request.Data {
		err := client.AddUser(ctx, xray.UserSpec{
			Protocol:   xray.UserProtocol(item.Type),
			Tag:        item.Tag,
			Username:   item.Username,
			Password:   item.Password,
			UUID:       item.UUID,
			Flow:       item.Flow,
			CipherType: item.CipherType,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			ctrl.logger.Warn("add xray user", "tag", item.Tag, "type", item.Type, "error", err)
			continue
		}
		success = true
		ctrl.state.AddUserToInbound(item.Tag, request.HashData.VlessUUID)
	}

	ctrl.writeSuccess(c, success, errIf(!success, firstErr))
}

func (ctrl HandlerController) AddUsers(c *gin.Context) {
	var request contracts.AddUsersRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}
	ctrl.state.AddKnownInboundTags(request.AffectedInboundTags...)

	client, err := ctrl.handlerClient()
	if err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}

	ctx, cancel := handlerContext(c)
	defer cancel()
	success, firstErr := false, error(nil)
	for _, user := range request.Users {
		for _, tag := range ctrl.state.KnownInboundTags() {
			ctrl.removeUser(ctx, client, tag, user.UserData.UserID)
			ctrl.state.RemoveUserFromInbound(tag, user.UserData.HashUUID)
		}

		for _, item := range user.InboundData {
			spec := bulkUserSpec(item, user.UserData)
			err := client.AddUser(ctx, spec)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				ctrl.logger.Warn("add xray user", "tag", item.Tag, "type", item.Type, "error", err)
				continue
			}
			success = true
			ctrl.state.AddUserToInbound(item.Tag, user.UserData.VlessUUID)
		}
	}

	ctrl.writeSuccess(c, success || len(request.Users) == 0, errIf(!success && len(request.Users) > 0, firstErr))
}

func (ctrl HandlerController) RemoveUser(c *gin.Context) {
	var request contracts.RemoveUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}
	tags := ctrl.state.KnownInboundTags()
	if len(tags) == 0 {
		ctrl.writeSuccess(c, true, nil)
		return
	}

	client, err := ctrl.handlerClient()
	if err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}

	ctx, cancel := handlerContext(c)
	defer cancel()
	success, firstErr := ctrl.removeFromAllKnownInbounds(ctx, client, request.Username, request.HashData.VlessUUID)
	ctrl.writeSuccess(c, success, firstErr)
}

func (ctrl HandlerController) RemoveUsers(c *gin.Context) {
	var request contracts.RemoveUsersRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}
	if len(ctrl.state.KnownInboundTags()) == 0 {
		ctrl.writeSuccess(c, true, nil)
		return
	}

	client, err := ctrl.handlerClient()
	if err != nil {
		ctrl.writeSuccess(c, false, err)
		return
	}

	ctx, cancel := handlerContext(c)
	defer cancel()
	success, firstErr := true, error(nil)
	for _, user := range request.Users {
		userSuccess, err := ctrl.removeFromAllKnownInbounds(ctx, client, user.UserID, user.HashUUID)
		if !userSuccess {
			success = false
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	ctrl.writeSuccess(c, success, firstErr)
}

func (ctrl HandlerController) GetInboundUsers(c *gin.Context) {
	var request contracts.InboundTagRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersResponse{Users: []contracts.InboundUser{}})
		return
	}
	client, err := ctrl.handlerClient()
	if err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersResponse{Users: []contracts.InboundUser{}})
		return
	}
	ctx, cancel := handlerContext(c)
	defer cancel()
	users, err := client.GetInboundUsers(ctx, request.Tag)
	if err != nil {
		ctrl.logger.Warn("get xray inbound users", "tag", request.Tag, "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersResponse{Users: []contracts.InboundUser{}})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersResponse{Users: contractInboundUsers(users)})
}

func (ctrl HandlerController) GetInboundUsersCount(c *gin.Context) {
	var request contracts.InboundTagRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersCountResponse{Count: 0})
		return
	}
	client, err := ctrl.handlerClient()
	if err != nil {
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersCountResponse{Count: 0})
		return
	}
	ctx, cancel := handlerContext(c)
	defer cancel()
	count, err := client.GetInboundUsersCount(ctx, request.Tag)
	if err != nil {
		ctrl.logger.Warn("get xray inbound users count", "tag", request.Tag, "error", err)
		httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersCountResponse{Count: 0})
		return
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.InboundUsersCountResponse{Count: count})
}

func (ctrl HandlerController) DropUsersConnections(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SimpleSuccess())
}

func (ctrl HandlerController) DropIPs(c *gin.Context) {
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.SimpleSuccess())
}

func (ctrl HandlerController) handlerClient() (xray.HandlerClient, error) {
	if ctrl.core == nil || !ctrl.core.IsRunning() {
		return nil, errors.New("xray core not running")
	}
	client := ctrl.core.Handler()
	if client == nil {
		return nil, errors.New("xray handler client is unavailable")
	}
	return client, nil
}

func (ctrl HandlerController) removeUser(ctx context.Context, client xray.HandlerClient, tag string, username string) error {
	err := client.RemoveUser(ctx, tag, username)
	if err != nil {
		ctrl.logger.Debug("remove xray user", "tag", tag, "error", err)
	}
	return err
}

func (ctrl HandlerController) removeFromAllKnownInbounds(ctx context.Context, client xray.HandlerClient, username string, userHash string) (bool, error) {
	success, firstErr := false, error(nil)
	for _, tag := range ctrl.state.KnownInboundTags() {
		err := ctrl.removeUser(ctx, client, tag, username)
		ctrl.state.RemoveUserFromInbound(tag, userHash)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		success = true
	}
	return success, firstErr
}

func (ctrl HandlerController) writeSuccess(c *gin.Context, success bool, err error) {
	var errMsg *string
	if err != nil {
		value := err.Error()
		errMsg = &value
	}
	httpapi.WriteEnvelope(c, http.StatusOK, contracts.GenericResponse{Success: success, Error: errMsg})
}

func handlerContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 10*time.Second)
}

func bulkUserSpec(item contracts.BulkUserInboundData, user contracts.BulkUserData) xray.UserSpec {
	spec := xray.UserSpec{
		Protocol: xray.UserProtocol(item.Type),
		Tag:      item.Tag,
		Username: user.UserID,
		Flow:     item.Flow,
	}
	switch item.Type {
	case string(xray.ProtocolTrojan):
		spec.Password = user.TrojanPassword
	case string(xray.ProtocolVLESS):
		spec.UUID = user.VlessUUID
	case string(xray.ProtocolShadowsocks):
		spec.Password = user.SSPassword
		spec.CipherType = 0
	case string(xray.ProtocolShadowsocks22):
		spec.Key = base64.StdEncoding.EncodeToString([]byte(user.SSPassword))
	case string(xray.ProtocolHysteria):
		spec.Password = user.VlessUUID
	}
	return spec
}

func contractInboundUsers(users []xray.InboundUser) []contracts.InboundUser {
	output := make([]contracts.InboundUser, 0, len(users))
	for _, user := range users {
		output = append(output, contracts.InboundUser{
			Username: user.Username,
			Email:    user.Email,
			Level:    user.Level,
		})
	}
	return output
}

func errIf(condition bool, err error) error {
	if condition {
		if err != nil {
			return err
		}
		return errors.New("xray handler operation failed")
	}
	return nil
}
