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

type ProjectEnvController struct {
	service *service.ProjectEnvService
}

func NewProjectEnvController(service *service.ProjectEnvService) *ProjectEnvController {
	return &ProjectEnvController{service: service}
}

func (ctl *ProjectEnvController) Get(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.service.Get(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load project env", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load project env", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load project env", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load project env", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "project env loaded", mapper.ToProjectEnvBundleResponse(*result))
}

func (ctl *ProjectEnvController) Upsert(c *gin.Context) {
	var req requestdto.UpsertProjectEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}
	claims := middleware.MustClaims(c)
	result, err := ctl.service.Upsert(mapper.ToUpsertProjectEnvCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to save project env", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to save project env", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to save project env", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to save project env", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "project env saved", mapper.ToProjectEnvBundleResponse(*result))
}

func (ctl *ProjectEnvController) Delete(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.service.Delete(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to clear project env", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to clear project env", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to clear project env", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to clear project env", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "project env cleared", mapper.ToProjectEnvBundleResponse(*result))
}
