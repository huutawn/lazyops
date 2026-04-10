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

type ProjectInternalServiceController struct {
	services *service.ProjectInternalServiceService
}

func NewProjectInternalServiceController(services *service.ProjectInternalServiceService) *ProjectInternalServiceController {
	return &ProjectInternalServiceController{services: services}
}

func (ctl *ProjectInternalServiceController) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.services.List(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load internal services", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load internal services", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load internal services", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load internal services", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "internal services loaded", mapper.ToProjectInternalServiceListResponse(*result))
}

func (ctl *ProjectInternalServiceController) Configure(c *gin.Context) {
	var req requestdto.ConfigureProjectInternalServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.services.Configure(mapper.ToConfigureProjectInternalServicesCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to configure internal services", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to configure internal services", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to configure internal services", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to configure internal services", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "internal services configured", mapper.ToProjectInternalServiceListResponse(*result))
}
