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

func (ctl *DeploymentController) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	items, err := ctl.deployments.List(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load deployments", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load deployments", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load deployments", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load deployments", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "deployments loaded", mapper.ToDeploymentListResponse(items))
}

func (ctl *DeploymentController) Get(c *gin.Context) {
	claims := middleware.MustClaims(c)
	record, err := ctl.deployments.Get(claims.UserID, claims.Role, c.Param("id"), c.Param("deployment_id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load deployment", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load deployment", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load deployment", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrDeploymentNotFound):
			response.Error(c, http.StatusNotFound, "failed to load deployment", "deployment_not_found", err.Error())
		case errors.Is(err, service.ErrRevisionNotFound):
			response.Error(c, http.StatusNotFound, "failed to load deployment", "revision_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load deployment", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "deployment loaded", mapper.ToDeploymentDetailResponse(*record))
}

func (ctl *DeploymentController) Act(c *gin.Context) {
	var req requestdto.DeploymentActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	record, err := ctl.deployments.Act(claims.UserID, claims.Role, c.Param("id"), c.Param("deployment_id"), req.Action)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "deployment action failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "deployment action failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "deployment action failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrDeploymentNotFound):
			response.Error(c, http.StatusNotFound, "deployment action failed", "deployment_not_found", err.Error())
		case errors.Is(err, service.ErrRevisionNotFound):
			response.Error(c, http.StatusNotFound, "deployment action failed", "revision_not_found", err.Error())
		case errors.Is(err, service.ErrInvalidRevisionStateTransition),
			errors.Is(err, service.ErrInvalidDeploymentStateTransition):
			response.Error(c, http.StatusConflict, "deployment action failed", "invalid_state_transition", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "deployment action failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "deployment action completed", mapper.ToDeploymentDetailResponse(*record))
}
