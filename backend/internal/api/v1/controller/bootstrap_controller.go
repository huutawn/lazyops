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

type BootstrapController struct {
	orchestrator *service.BootstrapOrchestrator
}

func NewBootstrapController(orchestrator *service.BootstrapOrchestrator) *BootstrapController {
	return &BootstrapController{orchestrator: orchestrator}
}

func (ctl *BootstrapController) Status(c *gin.Context) {
	if ctl.orchestrator == nil {
		response.Error(c, http.StatusNotImplemented, "bootstrap orchestration is not enabled", "not_enabled", nil)
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.orchestrator.GetStatus(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load bootstrap status", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load bootstrap status", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load bootstrap status", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load bootstrap status", "bootstrap_status_unavailable", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "bootstrap status loaded", mapper.ToBootstrapStatusResponse(*result))
}

func (ctl *BootstrapController) Auto(c *gin.Context) {
	if ctl.orchestrator == nil {
		response.Error(c, http.StatusNotImplemented, "bootstrap orchestration is not enabled", "not_enabled", nil)
		return
	}

	var req requestdto.BootstrapAutoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.orchestrator.AutoBootstrap(mapper.ToBootstrapAutoCommand(claims.UserID, claims.Role, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "auto bootstrap failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "auto bootstrap failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "auto bootstrap failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrRepoNotAccessible):
			response.Error(c, http.StatusForbidden, "auto bootstrap failed", "repo_not_accessible", err.Error())
		case errors.Is(err, service.ErrInvalidTrackedBranch):
			response.Error(c, http.StatusBadRequest, "auto bootstrap failed", "invalid_branch", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "auto bootstrap failed", "target_not_found", err.Error())
		case errors.Is(err, service.ErrTargetAccessDenied):
			response.Error(c, http.StatusForbidden, "auto bootstrap failed", "target_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "auto bootstrap failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusAccepted, "auto bootstrap accepted", mapper.ToBootstrapAutoAcceptedResponse(*result))
}

func (ctl *BootstrapController) OneClickDeploy(c *gin.Context) {
	if ctl.orchestrator == nil {
		response.Error(c, http.StatusNotImplemented, "bootstrap orchestration is not enabled", "not_enabled", nil)
		return
	}

	var req requestdto.BootstrapOneClickDeployRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
			return
		}
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.orchestrator.OneClickDeploy(mapper.ToBootstrapOneClickDeployCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "one-click deploy failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "one-click deploy failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "one-click deploy failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrRepoLinkNotFound):
			response.Error(c, http.StatusConflict, "one-click deploy failed", "repo_link_not_found", err.Error())
		case errors.Is(err, service.ErrUnknownTargetRef), errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "one-click deploy failed", "target_not_found", err.Error())
		case errors.Is(err, service.ErrInvalidDependencyMapping):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "invalid_dependency_mapping", err.Error())
		case errors.Is(err, service.ErrRuntimeModeMismatch):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "runtime_mode_mismatch", err.Error())
		case errors.Is(err, service.ErrSecretBearingConfig):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "secret_bearing_config", err.Error())
		case errors.Is(err, service.ErrHardCodedDeployAuthority):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "hard_coded_deploy_authority", err.Error())
		case errors.Is(err, service.ErrBlueprintNotFound):
			response.Error(c, http.StatusNotFound, "one-click deploy failed", "blueprint_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "one-click deploy failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "one-click deployment created", mapper.ToBootstrapOneClickDeployResponse(*result))
}
