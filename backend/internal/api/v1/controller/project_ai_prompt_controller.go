package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type ProjectAIPromptController struct {
	service *service.ProjectAIPromptService
}

func NewProjectAIPromptController(service *service.ProjectAIPromptService) *ProjectAIPromptController {
	return &ProjectAIPromptController{service: service}
}

func (ctl *ProjectAIPromptController) Get(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.service.Get(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load ai migration prompt", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load ai migration prompt", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load ai migration prompt", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load ai migration prompt", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "ai migration prompt loaded", mapper.ToProjectAIPromptResponse(*result))
}
