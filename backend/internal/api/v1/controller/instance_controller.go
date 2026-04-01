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

type InstanceController struct {
	instances *service.InstanceService
}

func NewInstanceController(instances *service.InstanceService) *InstanceController {
	return &InstanceController{instances: instances}
}

func (ctl *InstanceController) Create(c *gin.Context) {
	var req requestdto.CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.instances.Create(mapper.ToCreateInstanceCommand(claims.UserID, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidIP):
			response.Error(c, http.StatusBadRequest, "instance creation failed", "invalid_ip", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "instance creation failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrInstanceNameExists):
			response.Error(c, http.StatusConflict, "instance creation failed", "instance_name_exists", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "instance creation failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "instance created", mapper.ToCreateInstanceResponse(*result))
}

func (ctl *InstanceController) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.instances.List(claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "failed to load instances", "invalid_input", err.Error())
			return
		}

		response.Error(c, http.StatusInternalServerError, "failed to load instances", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "instances loaded", mapper.ToInstanceListResponse(*result))
}
