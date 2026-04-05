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

type DeploymentController struct {
	deployments *service.DeploymentService
	rollouts    *service.RolloutExecutionService
}

func NewDeploymentController(deployments *service.DeploymentService, rollouts *service.RolloutExecutionService) *DeploymentController {
	return &DeploymentController{deployments: deployments, rollouts: rollouts}
}

func (ctl *DeploymentController) Create(c *gin.Context) {
	var req requestdto.CreateDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.deployments.Create(mapper.ToCreateDeploymentCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "deployment create failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "deployment create failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "deployment create failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrBlueprintNotFound):
			response.Error(c, http.StatusNotFound, "deployment create failed", "blueprint_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "deployment create failed", "internal_error", err.Error())
		}
		return
	}

	payload := mapper.ToCreateDeploymentResponse(*result)
	if ctl.rollouts == nil {
		response.JSON(c, http.StatusCreated, "deployment created", payload)
		return
	}

	rolloutResult, rolloutErr := ctl.rollouts.StartDeployment(c.Request.Context(), result.Deployment.ProjectID, result.Deployment.ID)
	if rolloutErr == nil {
		response.JSONWithMeta(c, http.StatusCreated, "deployment created", payload, gin.H{
			"rollout":             "started",
			"agent_id":            rolloutResult.AgentID,
			"correlation_id":      rolloutResult.CorrelationID,
			"dispatched_commands": rolloutResult.DispatchedCommands,
		})
		return
	}

	switch {
	case errors.Is(rolloutErr, service.ErrRolloutArtifactPending),
		errors.Is(rolloutErr, service.ErrRolloutAgentUnavailable),
		errors.Is(rolloutErr, service.ErrRolloutUnsupportedTarget),
		errors.Is(rolloutErr, service.ErrRolloutAlreadyStarted):
		response.JSONWithMeta(c, http.StatusCreated, "deployment created", payload, gin.H{
			"rollout": "pending",
			"reason":  rolloutErr.Error(),
		})
	default:
		response.JSONWithMeta(c, http.StatusAccepted, "deployment created; rollout kickoff failed", payload, gin.H{
			"rollout": "failed_to_start",
			"reason":  rolloutErr.Error(),
		})
	}
}
