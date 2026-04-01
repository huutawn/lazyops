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

type DeploymentBindingController struct {
	bindings *service.DeploymentBindingService
}

func NewDeploymentBindingController(bindings *service.DeploymentBindingService) *DeploymentBindingController {
	return &DeploymentBindingController{bindings: bindings}
}

func (ctl *DeploymentBindingController) Create(c *gin.Context) {
	var req requestdto.CreateDeploymentBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.bindings.Create(mapper.ToCreateDeploymentBindingCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "deployment binding failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "deployment binding failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "deployment binding failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "deployment binding failed", "unknown_target", err.Error())
		case errors.Is(err, service.ErrTargetAccessDenied):
			response.Error(c, http.StatusForbidden, "deployment binding failed", "target_access_denied", err.Error())
		case errors.Is(err, service.ErrDuplicateTargetRef):
			response.Error(c, http.StatusConflict, "deployment binding failed", "duplicate_target_ref", err.Error())
		case errors.Is(err, service.ErrRuntimeModeMismatch):
			response.Error(c, http.StatusUnprocessableEntity, "deployment binding failed", "runtime_mode_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "deployment binding failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "deployment binding created", mapper.ToDeploymentBindingResponse(*result))
}
