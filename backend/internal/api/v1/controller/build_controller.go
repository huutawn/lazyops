package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
	"lazyops-server/pkg/logger"
)

type BuildController struct {
	callbacks *service.BuildCallbackService
}

func NewBuildController(callbacks *service.BuildCallbackService) *BuildController {
	return &BuildController{callbacks: callbacks}
}

func (ctl *BuildController) Callback(c *gin.Context) {
	var req requestdto.BuildCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid build callback payload", "invalid_payload", err.Error())
		return
	}

	result, err := ctl.callbacks.Handle(mapper.ToBuildCallbackCommand(req))
	if err != nil {
		logBuildCallbackOutcome(req, nil, err)
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "build callback rejected", "invalid_payload", err.Error())
		case errors.Is(err, service.ErrBuildJobNotFound):
			response.Error(c, http.StatusNotFound, "build callback rejected", "build_job_not_found", err.Error())
		case errors.Is(err, service.ErrBuildArtifactMismatch):
			response.Error(c, http.StatusConflict, "build callback rejected", "artifact_mismatch", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "build callback failed", "internal_error", err.Error())
		}
		return
	}

	logBuildCallbackOutcome(req, result, nil)
	response.JSON(c, http.StatusOK, "build callback accepted", mapper.ToBuildCallbackResponse(*result))
}

func logBuildCallbackOutcome(req requestdto.BuildCallbackRequest, result *service.BuildCallbackResult, err error) {
	args := []any{
		"build_job_id", req.BuildJobID,
		"project_id", req.ProjectID,
		"commit_sha", req.CommitSHA,
		"status", req.Status,
	}
	if result != nil {
		args = append(args,
			"build_job_status", result.BuildJob.Status,
			"revision_created", result.Revision != nil,
		)
	}
	if err != nil {
		args = append(args, "error", err.Error())
		logger.Warn("build_callback_failed", args...)
		return
	}
	logger.Info("build_callback_accepted", args...)
}
