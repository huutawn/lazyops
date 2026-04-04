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
}

func NewDeploymentController(deployments *service.DeploymentService) *DeploymentController {
	return &DeploymentController{deployments: deployments}
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

	response.JSON(c, http.StatusCreated, "deployment created", mapper.ToCreateDeploymentResponse(*result))
}
