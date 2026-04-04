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

type BlueprintController struct {
	blueprints *service.BlueprintService
}

func NewBlueprintController(blueprints *service.BlueprintService) *BlueprintController {
	return &BlueprintController{blueprints: blueprints}
}

func (ctl *BlueprintController) Compile(c *gin.Context) {
	var req requestdto.CompileBlueprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.blueprints.Compile(mapper.ToCompileBlueprintCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusUnprocessableEntity, "blueprint compile failed", "invalid_lazyops_yaml", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "blueprint compile failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "blueprint compile failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrRepoLinkNotFound):
			response.Error(c, http.StatusConflict, "blueprint compile failed", "repo_link_not_found", err.Error())
		case errors.Is(err, service.ErrUnknownTargetRef):
			response.Error(c, http.StatusNotFound, "blueprint compile failed", "unknown_target_ref", err.Error())
		case errors.Is(err, service.ErrInvalidDependencyMapping):
			response.Error(c, http.StatusUnprocessableEntity, "blueprint compile failed", "invalid_dependency_mapping", err.Error())
		case errors.Is(err, service.ErrSecretBearingConfig):
			response.Error(c, http.StatusUnprocessableEntity, "blueprint compile failed", "secret_bearing_config", err.Error())
		case errors.Is(err, service.ErrHardCodedDeployAuthority):
			response.Error(c, http.StatusUnprocessableEntity, "blueprint compile failed", "hard_coded_deploy_authority", err.Error())
		case errors.Is(err, service.ErrRuntimeModeMismatch):
			response.Error(c, http.StatusUnprocessableEntity, "blueprint compile failed", "runtime_mode_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "blueprint compile failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "blueprint compiled", mapper.ToCompileBlueprintResponse(*result))
}
