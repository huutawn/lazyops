package controller

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type InitContractController struct {
	initContracts *service.InitContractService
}

func NewInitContractController(initContracts *service.InitContractService) *InitContractController {
	return &InitContractController{initContracts: initContracts}
}

func (ctl *InitContractController) ValidateLazyopsYAML(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.initContracts.ValidateLazyopsYAML(service.ValidateLazyopsYAMLCommand{
		RequesterUserID: claims.UserID,
		RequesterRole:   claims.Role,
		ProjectID:       c.Param("id"),
		RawDocument:     raw,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusUnprocessableEntity, "lazyops.yaml validation failed", "invalid_lazyops_yaml", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "lazyops.yaml validation failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "lazyops.yaml validation failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrUnknownTargetRef):
			response.Error(c, http.StatusNotFound, "lazyops.yaml validation failed", "unknown_target_ref", err.Error())
		case errors.Is(err, service.ErrInvalidDependencyMapping):
			response.Error(c, http.StatusUnprocessableEntity, "lazyops.yaml validation failed", "invalid_dependency_mapping", err.Error())
		case errors.Is(err, service.ErrSecretBearingConfig):
			response.Error(c, http.StatusUnprocessableEntity, "lazyops.yaml validation failed", "secret_bearing_config", err.Error())
		case errors.Is(err, service.ErrHardCodedDeployAuthority):
			response.Error(c, http.StatusUnprocessableEntity, "lazyops.yaml validation failed", "hard_coded_deploy_authority", err.Error())
		case errors.Is(err, service.ErrRuntimeModeMismatch):
			response.Error(c, http.StatusUnprocessableEntity, "lazyops.yaml validation failed", "runtime_mode_mismatch", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "lazyops.yaml validation failed", "unknown_target", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "lazyops.yaml validation failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "lazyops.yaml contract validated", mapper.ToValidateLazyopsYAMLResponse(*result))
}
