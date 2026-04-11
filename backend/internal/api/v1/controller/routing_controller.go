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

type RoutingController struct {
	services *service.RoutingService
}

func NewRoutingController(services *service.RoutingService) *RoutingController {
	return &RoutingController{services: services}
}

func (ctl *RoutingController) Get(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.services.GetRouting(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load routing config", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load routing config", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load routing config", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load routing config", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "routing configuration loaded", mapper.ToProjectRoutingResponse(*result))
}

func (ctl *RoutingController) Update(c *gin.Context) {
	var req requestdto.UpdateRoutingPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.services.UpdateRouting(mapper.ToUpdateRoutingCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to update routing config", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to update routing config", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to update routing config", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to update routing config", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "routing configuration updated", mapper.ToProjectRoutingResponse(*result))
}
