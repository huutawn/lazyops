package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type ProjectController struct {
	projects       *service.ProjectService
	repoLinks      *service.ProjectRepoLinkService
	clusterNodes   *service.ClusterNodeService
	runtimeService *service.ProjectRuntimeService
	serviceActions *service.ProjectServiceActionService
}

func NewProjectController(projects *service.ProjectService, repoLinks *service.ProjectRepoLinkService) *ProjectController {
	return &ProjectController{
		projects:  projects,
		repoLinks: repoLinks,
	}
}

func (ctl *ProjectController) WithClusterNodeService(clusterNodes *service.ClusterNodeService) *ProjectController {
	ctl.clusterNodes = clusterNodes
	return ctl
}

func (ctl *ProjectController) WithRuntimeService(runtimeService *service.ProjectRuntimeService) *ProjectController {
	ctl.runtimeService = runtimeService
	return ctl
}

func (ctl *ProjectController) WithServiceActionService(serviceActions *service.ProjectServiceActionService) *ProjectController {
	ctl.serviceActions = serviceActions
	return ctl
}

func (ctl *ProjectController) Create(c *gin.Context) {
	var req requestdto.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.projects.Create(mapper.ToCreateProjectCommand(claims.UserID, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "project creation failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectSlugExists):
			response.Error(c, http.StatusConflict, "project creation failed", "project_slug_exists", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "project creation failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "project created", mapper.ToProjectSummaryResponse(*result))
}

func (ctl *ProjectController) List(c *gin.Context) {
	claims := middleware.MustClaims(c)
	items, err := ctl.projects.List(claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "failed to load projects", "invalid_input", err.Error())
			return
		}

		response.Error(c, http.StatusInternalServerError, "failed to load projects", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "projects loaded", mapper.ToProjectListResponse(items))
}

func (ctl *ProjectController) ListServices(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.projects.ListServices(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load project services", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load project services", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load project services", "ownership_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load project services", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "project services loaded", mapper.ToProjectServiceListResponse(*result))
}

func (ctl *ProjectController) ConfigureServices(c *gin.Context) {
	var req requestdto.ConfigureProjectServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.projects.ConfigureServices(
		mapper.ToConfigureProjectServicesCommand(claims.UserID, claims.Role, c.Param("id"), req),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to configure project services", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to configure project services", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to configure project services", "ownership_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to configure project services", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "project services configured", mapper.ToProjectServiceListResponse(*result))
}

func (ctl *ProjectController) ListPlacementNodes(c *gin.Context) {
	if ctl.clusterNodes == nil {
		response.JSON(c, http.StatusOK, "placement nodes loaded", mapper.ToPlacementNodeListResponse(service.PlacementNodeListResult{Items: []service.ClusterNodeRecord{}}))
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.clusterNodes.ListPlacementNodes(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load placement nodes", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load placement nodes", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load placement nodes", "ownership_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load placement nodes", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "placement nodes loaded", mapper.ToPlacementNodeListResponse(*result))
}

func (ctl *ProjectController) GetRuntimeSummary(c *gin.Context) {
	if ctl.runtimeService == nil {
		response.Error(c, http.StatusNotImplemented, "runtime summary is not enabled", "not_enabled", nil)
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.runtimeService.Get(c.Request.Context(), claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load project runtime", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load project runtime", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load project runtime", "ownership_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load project runtime", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "project runtime loaded", mapper.ToProjectRuntimeSummaryResponse(*result))
}

func (ctl *ProjectController) LinkRepo(c *gin.Context) {
	var req requestdto.LinkProjectRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.repoLinks.LinkRepository(
		mapper.ToCreateProjectRepoLinkCommand(claims.UserID, claims.Role, c.Param("id"), req),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "repo link failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrInvalidTrackedBranch):
			response.Error(c, http.StatusBadRequest, "repo link failed", "invalid_branch", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "repo link failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "repo link failed", "ownership_mismatch", err.Error())
		case errors.Is(err, service.ErrRepoNotAccessible):
			response.Error(c, http.StatusForbidden, "repo link failed", "repo_not_accessible", err.Error())
		case errors.Is(err, service.ErrRepoLinkConflict):
			response.Error(c, http.StatusConflict, "repo link failed", "repo_link_conflict", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "repo link failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "repo linked", mapper.ToProjectRepoLinkResponse(*result))
}

func (ctl *ProjectController) GetRepoLink(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.repoLinks.GetProjectLink(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load repo link", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load repo link", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load repo link", "ownership_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load repo link", "internal_error", err.Error())
		}
		return
	}

	var payload *responsedto.ProjectRepoLinkResponse
	if result != nil {
		mapped := mapper.ToProjectRepoLinkResponse(*result)
		payload = &mapped
	}

	response.JSON(c, http.StatusOK, "repo link loaded", payload)
}

func (ctl *ProjectController) ActOnService(c *gin.Context) {
	if ctl.serviceActions == nil {
		response.Error(c, http.StatusNotImplemented, "service actions are not enabled", "not_enabled", nil)
		return
	}

	var req requestdto.ProjectServiceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.serviceActions.Act(c.Request.Context(), claims.UserID, claims.Role, c.Param("id"), c.Param("service_id"), req.Action)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "service action failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "service action failed", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "service action failed", "ownership_mismatch", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "service action failed", "target_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "service action failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "service action completed", mapper.ToProjectServiceActionResponse(*result))
}
