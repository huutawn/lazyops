package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type AgentRuntimeController struct {
	enrollment *service.AgentEnrollmentService
}

func NewAgentRuntimeController(enrollment *service.AgentEnrollmentService) *AgentRuntimeController {
	return &AgentRuntimeController{enrollment: enrollment}
}

func (ctl *AgentRuntimeController) Enroll(c *gin.Context) {
	var req requestdto.EnrollAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	result, err := ctl.enrollment.Enroll(mapper.ToAgentEnrollmentCommand(req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "agent enrollment failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrBootstrapTokenUnknown):
			response.Error(c, http.StatusUnauthorized, "agent enrollment failed", "invalid_token", nil)
		case errors.Is(err, service.ErrBootstrapTokenExpired):
			response.Error(c, http.StatusUnauthorized, "agent enrollment failed", "expired_token", nil)
		case errors.Is(err, service.ErrBootstrapTokenReused):
			response.Error(c, http.StatusUnauthorized, "agent enrollment failed", "reused_bootstrap_token", nil)
		case errors.Is(err, service.ErrBootstrapOwnershipMismatch):
			response.Error(c, http.StatusForbidden, "agent enrollment failed", "ownership_mismatch", nil)
		default:
			response.Error(c, http.StatusInternalServerError, "agent enrollment failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "agent enrolled", mapper.ToAgentEnrollmentResponse(*result))
}

func (ctl *AgentRuntimeController) Heartbeat(c *gin.Context) {
	var req requestdto.AgentHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	command := mapper.ToAgentHeartbeatCommand(req)
	command.UserID = claims.UserID
	command.AgentID = claims.AgentID
	command.InstanceID = claims.InstanceID
	if req.AgentID != "" && req.AgentID != claims.AgentID {
		response.Error(c, http.StatusForbidden, "agent heartbeat failed", "ownership_mismatch", nil)
		return
	}

	result, err := ctl.enrollment.Heartbeat(command)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "agent heartbeat failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrAgentNotFound):
			response.Error(c, http.StatusNotFound, "agent heartbeat failed", "agent_not_found", nil)
		case errors.Is(err, service.ErrBootstrapOwnershipMismatch):
			response.Error(c, http.StatusForbidden, "agent heartbeat failed", "ownership_mismatch", nil)
		default:
			response.Error(c, http.StatusInternalServerError, "agent heartbeat failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "agent heartbeat accepted", mapper.ToAgentHeartbeatResponse(*result))
}
