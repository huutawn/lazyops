package controller

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/config"
	"lazyops-server/internal/service"
)

type GitHubController struct {
	installations *service.GitHubInstallationService
	cfg           config.Config
}

func NewGitHubController(installations *service.GitHubInstallationService, cfg config.Config) *GitHubController {
	return &GitHubController{
		installations: installations,
		cfg:           cfg,
	}
}

func (ctl *GitHubController) SyncInstallations(c *gin.Context) {
	var req requestdto.SyncGitHubInstallationsRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.installations.SyncInstallations(c.Request.Context(), mapper.ToSyncGitHubInstallationsCommand(claims.UserID, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "github installation sync failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrGitHubIdentityRequired):
			response.Error(c, http.StatusForbidden, "github installation sync failed", "github_identity_required", nil)
		case errors.Is(err, service.ErrGitHubProviderError):
			response.Error(c, http.StatusBadGateway, "github installation sync failed", "provider_error", nil)
		default:
			response.Error(c, http.StatusInternalServerError, "github installation sync failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "github installations synced", mapper.ToGitHubInstallationSyncResponse(*result))
}

func (ctl *GitHubController) ListRepos(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.installations.ListRepos(claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "github repo discovery failed", "invalid_input", err.Error())
			return
		}

		response.Error(c, http.StatusInternalServerError, "github repo discovery failed", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "github repos loaded", mapper.ToGitHubRepositoryListResponse(*result))
}

func (ctl *GitHubController) AppConfig(c *gin.Context) {
	response.JSON(c, http.StatusOK, "github app config loaded", mapper.ToGitHubAppConfigResponse(ctl.cfg.GitHubApp))
}
