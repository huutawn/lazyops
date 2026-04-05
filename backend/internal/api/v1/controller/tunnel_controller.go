package controller

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/service"
)

const defaultTunnelTTL = 30 * time.Minute

type TunnelController struct {
	projects service.ProjectStore
	bindings service.DeploymentBindingStore
	tunnels  service.TunnelSessionStore
	mesh     *service.MeshPlanningService
}

func NewTunnelController(
	projects service.ProjectStore,
	bindings service.DeploymentBindingStore,
	tunnels service.TunnelSessionStore,
	mesh *service.MeshPlanningService,
) *TunnelController {
	return &TunnelController{
		projects: projects,
		bindings: bindings,
		tunnels:  tunnels,
		mesh:     mesh,
	}
}

func (ctl *TunnelController) CreateDBSession(c *gin.Context) {
	ctl.createSession(c, service.TunnelSessionTypeDB)
}

func (ctl *TunnelController) CreateTCPSession(c *gin.Context) {
	ctl.createSession(c, service.TunnelSessionTypeTCP)
}

func (ctl *TunnelController) CloseSession(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	if sessionID == "" {
		response.Error(c, http.StatusBadRequest, "failed to close tunnel session", "invalid_input", "session id is required")
		return
	}

	session, err := ctl.tunnels.GetByID(sessionID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to close tunnel session", "internal_error", err.Error())
		return
	}
	if session == nil {
		response.Error(c, http.StatusNotFound, "failed to close tunnel session", "tunnel_session_not_found", service.ErrTunnelSessionNotFound.Error())
		return
	}

	claims := middleware.MustClaims(c)
	if _, err := resolveProjectForClaims(ctl.projects, claims, session.ProjectID); err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to close tunnel session", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to close tunnel session", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to close tunnel session", "internal_error", err.Error())
		}
		return
	}

	record, err := ctl.mesh.CloseTunnelSession(c.Request.Context(), sessionID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTunnelSessionNotFound):
			response.Error(c, http.StatusNotFound, "failed to close tunnel session", "tunnel_session_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to close tunnel session", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "tunnel session closed", gin.H{
		"session_id": record.ID,
		"status":     record.Status,
	})
}

func (ctl *TunnelController) createSession(c *gin.Context, sessionType string) {
	var req requestdto.CreateTunnelSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	project, err := resolveProjectForClaims(ctl.projects, claims, req.ProjectID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to create tunnel session", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to create tunnel session", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to create tunnel session", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to create tunnel session", "internal_error", err.Error())
		}
		return
	}

	binding, err := ctl.bindings.GetByTargetRefForProject(project.ID, strings.TrimSpace(req.TargetRef))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to create tunnel session", "internal_error", err.Error())
		return
	}
	if binding == nil {
		response.Error(c, http.StatusNotFound, "failed to create tunnel session", "deployment_binding_not_found", "deployment binding target_ref was not found for this project")
		return
	}
	if binding.TargetKind != "instance" {
		response.Error(
			c,
			http.StatusUnprocessableEntity,
			"failed to create tunnel session",
			"unsupported_tunnel_target",
			fmt.Sprintf("tunnel sessions currently support only instance targets, got %q", binding.TargetKind),
		)
		return
	}

	localPort := req.LocalPort
	if localPort <= 0 {
		response.Error(c, http.StatusBadRequest, "failed to create tunnel session", "invalid_input", "local_port must be greater than zero")
		return
	}

	remote := strings.TrimSpace(req.Remote)
	remotePort, err := parseTunnelRemotePort(remote)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "failed to create tunnel session", "invalid_input", err.Error())
		return
	}

	ttl, err := parseTunnelTTL(req.Timeout)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "failed to create tunnel session", "invalid_input", err.Error())
		return
	}

	record, err := ctl.mesh.CreateTunnelSession(
		c.Request.Context(),
		project.ID,
		binding.TargetKind,
		binding.TargetID,
		sessionType,
		localPort,
		remotePort,
		ttl,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "failed to create tunnel session", "target_not_found", err.Error())
		case errors.Is(err, service.ErrTargetOffline):
			response.Error(c, http.StatusConflict, "failed to create tunnel session", "target_offline", err.Error())
		case errors.Is(err, service.ErrTunnelSessionPortConflict):
			response.Error(c, http.StatusConflict, "failed to create tunnel session", "port_conflict", err.Error())
		case errors.Is(err, service.ErrTunnelSessionCloseFailed):
			response.Error(c, http.StatusConflict, "failed to create tunnel session", "session_cleanup_failed", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to create tunnel session", "invalid_input", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to create tunnel session", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "tunnel session created", gin.H{
		"session_id": record.ID,
		"local_port": record.LocalPort,
		"remote":     remote,
		"status":     record.Status,
		"expires_at": record.ExpiresAt.Format(time.RFC3339),
	})
}

func parseTunnelRemotePort(remote string) (int, error) {
	if remote == "" {
		return 0, errors.New("remote is required and must use host:port format")
	}

	_, port, err := net.SplitHostPort(remote)
	if err != nil {
		return 0, fmt.Errorf("remote %q must use host:port format", remote)
	}

	value, err := strconv.Atoi(port)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("remote %q must contain a valid port", remote)
	}

	return value, nil
}

func parseTunnelTTL(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultTunnelTTL, nil
	}

	ttl, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("timeout %q is invalid", raw)
	}
	if ttl <= 0 {
		return 0, errors.New("timeout must be greater than zero")
	}

	return ttl, nil
}
