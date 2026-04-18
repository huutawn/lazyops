package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
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
	var req requestdto.ConfigureProjectInternalServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	_, err := ctl.services.Configure(mapper.ToConfigureProjectInternalServicesCommand(claims.UserID, claims.Role, c.Param("id"), req))
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

	response.JSON(c, http.StatusOK, "internal services configured", mapper.ToProjectInternalServiceListResponse(*result))
}

func (ctl *ProjectInternalServiceController) listFromUnifiedInventory(userID, role, projectID string) (*service.ProjectInternalServiceListResult, error) {
	if ctl != nil && ctl.projectServices != nil {
		result, err := ctl.projectServices.ListServices(userID, role, projectID)
		if err != nil {
			return nil, err
		}
		return filterUnifiedInternalServices(result), nil
	}
	return ctl.services.List(userID, role, projectID)
}

func filterUnifiedInternalServices(result *service.ProjectServiceListResult) *service.ProjectInternalServiceListResult {
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
			LocalEndpoint: localEndpointForPort(port),
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		})
	}

	return &service.ProjectInternalServiceListResult{Items: items}
}

func localEndpointForPort(port int) string {
	if port <= 0 {
		return "localhost"
	}
	return "localhost:" + strconv.Itoa(port)
}
