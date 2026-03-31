package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

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
	agents, err := ctl.agents.List()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list agents", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "agents loaded", mapper.ToAgentListResponse(agents))
}

func (ctl *AgentController) Create(c *gin.Context) {
	var req requestdto.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", err.Error())
		return
	}

	agent, err := ctl.agents.Create(mapper.ToCreateAgentCommand(req))
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "failed to create agent", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to create agent", err.Error())
		return
	}

	response.JSON(c, http.StatusCreated, "agent created", mapper.ToAgentResponse(*agent))
}

func (ctl *AgentController) UpdateStatus(c *gin.Context) {
	var req requestdto.UpdateAgentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", err.Error())
		return
	}

	source := req.Source
	if source == "" {
		source = "http"
	}

	command := mapper.ToUpdateAgentStatusCommand(c.Param("agentID"), req)
	command.Source = source

	agent, err := ctl.agents.UpdateStatus(command)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "failed to update agent status", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to update agent status", err.Error())
		return
	}

	if err := ctl.hub.Broadcast(mapper.ToAgentRealtimeEventResponse(ctl.agents.BuildRealtimeEvent(*agent, source))); err != nil {
		hub.LogBroadcastFailure(err)
	}

	response.JSON(c, http.StatusOK, "agent status updated", mapper.ToAgentResponse(*agent))
}
