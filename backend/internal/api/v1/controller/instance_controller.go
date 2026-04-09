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
	instances     *service.InstanceService
	sshInstallSvc *service.InstanceSSHInstallService
	bootstrap     *service.BootstrapOrchestrator
}

func NewInstanceController(instances *service.InstanceService, sshInstallSvc *service.InstanceSSHInstallService) *InstanceController {
	return &InstanceController{
		instances:     instances,
		sshInstallSvc: sshInstallSvc,
	}
}

func (ctl *InstanceController) WithBootstrapOrchestrator(bootstrap *service.BootstrapOrchestrator) *InstanceController {
	ctl.bootstrap = bootstrap
	return ctl
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

	if ctl.bootstrap != nil {
		_ = ctl.bootstrap.OnInventoryChanged(claims.UserID)
	}
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

func (ctl *InstanceController) IssueBootstrapToken(c *gin.Context) {
	instanceID := c.Param("id")
	claims := middleware.MustClaims(c)

	issue, err := ctl.instances.IssueBootstrapToken(claims.UserID, instanceID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "bootstrap token issue failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrInstanceNotFound):
			response.Error(c, http.StatusNotFound, "bootstrap token issue failed", "instance_not_found", err.Error())
		case errors.Is(err, service.ErrInstanceBootstrapNotAllowed):
			response.Error(c, http.StatusConflict, "bootstrap token issue failed", "bootstrap_not_allowed", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "bootstrap token issue failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "bootstrap token issued", mapper.ToBootstrapTokenIssueResponse(*issue))
}

func (ctl *InstanceController) InstallAgentViaSSH(c *gin.Context) {
	if ctl.sshInstallSvc == nil {
		response.Error(c, http.StatusNotImplemented, "instance ssh install is not enabled", "not_enabled", nil)
		return
	}

	instanceID := c.Param("id")
	var req requestdto.InstallInstanceAgentSSHRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.sshInstallSvc.Install(c.Request.Context(), mapper.ToInstallInstanceAgentSSHCommand(claims.UserID, instanceID, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "ssh install failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrInstanceNotFound):
			response.Error(c, http.StatusNotFound, "ssh install failed", "instance_not_found", err.Error())
		case errors.Is(err, service.ErrInstanceBootstrapNotAllowed):
			response.Error(c, http.StatusConflict, "ssh install failed", "bootstrap_not_allowed", err.Error())
		case errors.Is(err, service.ErrSSHAuthenticationRequired):
			response.Error(c, http.StatusBadRequest, "ssh install failed", "ssh_auth_required", err.Error())
		case errors.Is(err, service.ErrSSHConnectionFailed):
			response.Error(c, http.StatusBadGateway, "ssh install failed", "ssh_connection_failed", err.Error())
		case errors.Is(err, service.ErrSSHExecutionFailed):
			response.Error(c, http.StatusBadGateway, "ssh install failed", "ssh_execution_failed", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "ssh install failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "ssh install started", mapper.ToInstallInstanceAgentSSHResponse(*result))

	if ctl.bootstrap != nil {
		_ = ctl.bootstrap.OnInventoryChanged(claims.UserID)
	}
}
