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

type ProjectController struct {
	projects  *service.ProjectService
	repoLinks *service.ProjectRepoLinkService
}

func NewProjectController(projects *service.ProjectService, repoLinks *service.ProjectRepoLinkService) *ProjectController {
	return &ProjectController{
		projects:  projects,
		repoLinks: repoLinks,
	}
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
