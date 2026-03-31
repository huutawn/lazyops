package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/hub"
	"lazyops-server/internal/service"
)

type AgentController struct {
	agents *service.AgentService
	hub    *hub.Hub
}

func NewAgentController(agents *service.AgentService, wsHub *hub.Hub) *AgentController {
	return &AgentController{
		agents: agents,
		hub:    wsHub,
	}
}

func (ctl *AgentController) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	agents, err := ctl.agents.List(claims.UserID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list agents", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "agents loaded", mapper.ToAgentListResponse(agents))
}

func (ctl *AgentController) Create(c *gin.Context) {
	var req requestdto.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	command := mapper.ToCreateAgentCommand(req)
	command.UserID = claims.UserID

	agent, err := ctl.agents.Create(command)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "failed to create agent", "invalid_input", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to create agent", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusCreated, "agent created", mapper.ToAgentResponse(*agent))
}

func (ctl *AgentController) UpdateStatus(c *gin.Context) {
	var req requestdto.UpdateAgentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	source := req.Source
	if source == "" {
		source = "http"
	}

	command := mapper.ToUpdateAgentStatusCommand(c.Param("agentID"), req)
	claims := middleware.MustClaims(c)
	command.UserID = claims.UserID
	command.Source = source

	agent, err := ctl.agents.UpdateStatus(command)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to update agent status", "invalid_input", err.Error())
			return
		case errors.Is(err, service.ErrAgentNotFound):
			response.Error(c, http.StatusNotFound, "failed to update agent status", "agent_not_found", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to update agent status", "internal_error", err.Error())
		return
	}

	if err := ctl.hub.BroadcastToUser(agent.UserID, mapper.ToAgentRealtimeEventResponse(ctl.agents.BuildRealtimeEvent(*agent, source))); err != nil {
		hub.LogBroadcastFailure(err)
	}

	response.JSON(c, http.StatusOK, "agent status updated", mapper.ToAgentResponse(*agent))
}
