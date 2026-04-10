package controller

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type BootstrapController struct {
	orchestrator *service.BootstrapOrchestrator
	instances    *service.InstanceService
	sshInstall   *service.InstanceSSHInstallService
}

func NewBootstrapController(
	orchestrator *service.BootstrapOrchestrator,
	instances *service.InstanceService,
	sshInstall *service.InstanceSSHInstallService,
) *BootstrapController {
	return &BootstrapController{
		orchestrator: orchestrator,
		instances:    instances,
		sshInstall:   sshInstall,
	}
}

func (ctl *BootstrapController) Status(c *gin.Context) {
	if ctl.orchestrator == nil {
		response.Error(c, http.StatusNotImplemented, "bootstrap orchestration is not enabled", "not_enabled", nil)
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.orchestrator.GetStatus(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load bootstrap status", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load bootstrap status", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load bootstrap status", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load bootstrap status", "bootstrap_status_unavailable", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "bootstrap status loaded", mapper.ToBootstrapStatusResponse(*result))
}

func (ctl *BootstrapController) Auto(c *gin.Context) {
	if ctl.orchestrator == nil {
		response.Error(c, http.StatusNotImplemented, "bootstrap orchestration is not enabled", "not_enabled", nil)
		return
	}

	var req requestdto.BootstrapAutoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.orchestrator.AutoBootstrap(mapper.ToBootstrapAutoCommand(claims.UserID, claims.Role, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "auto bootstrap failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "auto bootstrap failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "auto bootstrap failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrRepoNotAccessible):
			response.Error(c, http.StatusForbidden, "auto bootstrap failed", "repo_not_accessible", err.Error())
		case errors.Is(err, service.ErrInvalidTrackedBranch):
			response.Error(c, http.StatusBadRequest, "auto bootstrap failed", "invalid_branch", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "auto bootstrap failed", "target_not_found", err.Error())
		case errors.Is(err, service.ErrTargetAccessDenied):
			response.Error(c, http.StatusForbidden, "auto bootstrap failed", "target_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "auto bootstrap failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusAccepted, "auto bootstrap accepted", mapper.ToBootstrapAutoAcceptedResponse(*result))
}

func (ctl *BootstrapController) OneClickDeploy(c *gin.Context) {
	if ctl.orchestrator == nil {
		response.Error(c, http.StatusNotImplemented, "bootstrap orchestration is not enabled", "not_enabled", nil)
		return
	}

	var req requestdto.BootstrapOneClickDeployRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
			return
		}
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.orchestrator.OneClickDeploy(mapper.ToBootstrapOneClickDeployCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "one-click deploy failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "one-click deploy failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "one-click deploy failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrRepoLinkNotFound):
			response.Error(c, http.StatusConflict, "one-click deploy failed", "repo_link_not_found", err.Error())
		case errors.Is(err, service.ErrUnknownTargetRef), errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "one-click deploy failed", "target_not_found", err.Error())
		case errors.Is(err, service.ErrInvalidDependencyMapping):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "invalid_dependency_mapping", err.Error())
		case errors.Is(err, service.ErrRuntimeModeMismatch):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "runtime_mode_mismatch", err.Error())
		case errors.Is(err, service.ErrSecretBearingConfig):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "secret_bearing_config", err.Error())
		case errors.Is(err, service.ErrHardCodedDeployAuthority):
			response.Error(c, http.StatusUnprocessableEntity, "one-click deploy failed", "hard_coded_deploy_authority", err.Error())
		case errors.Is(err, service.ErrBlueprintNotFound):
			response.Error(c, http.StatusNotFound, "one-click deploy failed", "blueprint_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "one-click deploy failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "one-click deployment created", mapper.ToBootstrapOneClickDeployResponse(*result))
}

func (ctl *BootstrapController) ConnectInfraSSH(c *gin.Context) {
	if ctl.orchestrator == nil || ctl.instances == nil || ctl.sshInstall == nil {
		response.Error(c, http.StatusNotImplemented, "infra ssh orchestration is not enabled", "not_enabled", nil)
		return
	}

	var req requestdto.BootstrapConnectInfraSSHRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	projectID := strings.TrimSpace(c.Param("id"))
	if _, err := ctl.orchestrator.GetStatus(claims.UserID, claims.Role, projectID); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "infra connect failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "infra connect failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "infra connect failed", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "infra connect failed", "internal_error", err.Error())
		}
		return
	}

	sshHost := strings.TrimSpace(req.SSHHost)
	publicIP := strings.TrimSpace(req.PublicIP)
	privateIP := strings.TrimSpace(req.PrivateIP)
	if publicIP == "" && privateIP == "" {
		if parsed := net.ParseIP(sshHost); parsed != nil {
			publicIP = sshHost
		}
	}

	instanceName := strings.TrimSpace(req.InstanceName)
	if instanceName == "" {
		instanceName = "srv-" + sanitizeInstanceNameSuffix(sshHost)
	}

	createResult, err := ctl.instances.Create(service.CreateInstanceCommand{
		UserID:    claims.UserID,
		Name:      instanceName,
		PublicIP:  publicIP,
		PrivateIP: privateIP,
		Labels:    req.Labels,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidIP):
			response.Error(c, http.StatusBadRequest, "infra connect failed", "invalid_ip", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "infra connect failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrInstanceNameExists):
			response.Error(c, http.StatusConflict, "infra connect failed", "instance_name_exists", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "infra connect failed", "internal_error", err.Error())
		}
		return
	}

	controlPlaneURL := strings.TrimSpace(req.ControlPlaneURL)
	if controlPlaneURL == "" {
		controlPlaneURL = defaultControlPlaneURL(c)
	}

	installResult, err := ctl.sshInstall.Install(c.Request.Context(), service.InstallInstanceAgentSSHCommand{
		UserID:             claims.UserID,
		ProjectID:          projectID,
		InstanceID:         createResult.Instance.ID,
		Host:               sshHost,
		Port:               req.SSHPort,
		Username:           strings.TrimSpace(req.SSHUsername),
		Password:           req.SSHPassword,
		PrivateKey:         req.SSHPrivateKey,
		HostKeyFingerprint: req.SSHHostKeyFingerprint,
		ControlPlaneURL:    controlPlaneURL,
		AgentImage:         strings.TrimSpace(req.AgentImage),
		ContainerName:      strings.TrimSpace(req.ContainerName),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "infra connect failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrInstanceNotFound):
			response.Error(c, http.StatusNotFound, "infra connect failed", "instance_not_found", err.Error())
		case errors.Is(err, service.ErrInstanceBootstrapNotAllowed):
			response.Error(c, http.StatusConflict, "infra connect failed", "bootstrap_not_allowed", err.Error())
		case errors.Is(err, service.ErrSSHAuthenticationRequired):
			response.Error(c, http.StatusBadRequest, "infra connect failed", "ssh_auth_required", err.Error())
		case errors.Is(err, service.ErrSSHConnectionFailed):
			response.Error(c, http.StatusBadGateway, "infra connect failed", "ssh_connection_failed", err.Error())
		case errors.Is(err, service.ErrSSHExecutionFailed):
			response.Error(c, http.StatusBadGateway, "infra connect failed", "ssh_execution_failed", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "infra connect failed", "internal_error", err.Error())
		}
		return
	}

	autoResult, err := ctl.orchestrator.AutoBootstrap(service.BootstrapAutoCommand{
		RequesterUserID: claims.UserID,
		RequesterRole:   claims.Role,
		ProjectID:       projectID,
		InstanceID:      createResult.Instance.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "infra connect failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "infra connect failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "infra connect failed", "project_access_denied", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "infra connect failed", "target_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "infra connect failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(
		c,
		http.StatusCreated,
		"infra connected via ssh",
		mapper.ToBootstrapConnectInfraSSHResponse(projectID, createResult.Instance, *installResult, *autoResult),
	)
}

func sanitizeInstanceNameSuffix(input string) string {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" {
		return "auto"
	}
	var b strings.Builder
	lastHyphen := false
	for _, r := range normalized {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_' || r == '.' || r == ':':
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	value := strings.Trim(b.String(), "-")
	if value == "" {
		return "auto"
	}
	if len(value) > 40 {
		return value[:40]
	}
	return value
}

func defaultControlPlaneURL(c *gin.Context) string {
	proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if proto != "" {
		proto = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host != "" {
		host = strings.TrimSpace(strings.Split(host, ",")[0])
	}
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		host = "localhost:8080"
	}

	return strings.ToLower(proto) + "://" + host
}
