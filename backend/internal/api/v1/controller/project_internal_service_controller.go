package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type ProjectInternalServiceController struct {
	services        *service.ProjectInternalServiceService
	projectServices *service.ProjectService
}

func NewProjectInternalServiceController(services *service.ProjectInternalServiceService, projectServices *service.ProjectService) *ProjectInternalServiceController {
	return &ProjectInternalServiceController{
		services:        services,
		projectServices: projectServices,
	}
}

func (ctl *ProjectInternalServiceController) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.listFromUnifiedInventory(claims.UserID, claims.Role, c.Param("id"))
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
	response.Error(
		c,
		http.StatusConflict,
		"internal services are managed through the unified service inventory",
		"service_first_required",
		"use PUT /projects/:id/services to create or update internal services",
	)
}

func (ctl *ProjectInternalServiceController) listFromUnifiedInventory(userID, role, projectID string) (*service.ProjectInternalServiceListResult, error) {
	if ctl != nil && ctl.projectServices != nil {
		runtimeMode := ""
		summary, err := ctl.projectServices.GetSummary(userID, role, projectID)
		if err != nil {
			return nil, err
		}
		if summary != nil {
			runtimeMode = summary.RuntimeMode
		}
		result, err := ctl.projectServices.ListServices(userID, role, projectID)
		if err != nil {
			return nil, err
		}
		return filterUnifiedInternalServices(result, runtimeMode), nil
	}
	return ctl.services.List(userID, role, projectID)
}

func filterUnifiedInternalServices(result *service.ProjectServiceListResult, runtimeMode string) *service.ProjectInternalServiceListResult {
	if result == nil {
		return &service.ProjectInternalServiceListResult{Items: []service.ProjectInternalServiceRecord{}}
	}

	items := make([]service.ProjectInternalServiceRecord, 0, len(result.Items))
	for _, item := range result.Items {
		if item.SourceType != "internal" {
			continue
		}
		port := item.ServicePort
		if port <= 0 {
			port = item.TargetPort
		}
		protocol := "tcp"
		if rawProtocol, ok := item.Healthcheck["protocol"].(string); ok && rawProtocol != "" {
			protocol = rawProtocol
		}
		kind := item.Kind
		if kind == "" {
			kind = item.Name
		}
		items = append(items, service.ProjectInternalServiceRecord{
			ID:            item.ID,
			ProjectID:     item.ProjectID,
			Kind:          kind,
			Alias:         kind,
			Protocol:      protocol,
			Port:          port,
			LocalEndpoint: localEndpointForService(item.Name, kind, port, runtimeMode),
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		})
	}

	return &service.ProjectInternalServiceListResult{Items: items}
}

func localEndpointForService(name, kind string, port int, runtimeMode string) string {
	host := "localhost"
	if strings.TrimSpace(runtimeMode) == "distributed-k3s" {
		host = strings.TrimSpace(name)
		if host == "" {
			host = strings.TrimSpace(kind)
		}
		if host == "" {
			host = "internal-service"
		}
	}
	return localEndpointForHost(host, port)
}

func localEndpointForHost(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "localhost"
	}
	if port <= 0 {
		return host
	}
	return host + ":" + strconv.Itoa(port)
}

func localEndpointForPort(port int) string {
	return localEndpointForHost("localhost", port)
}
