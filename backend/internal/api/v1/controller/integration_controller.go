package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
	"lazyops-server/pkg/logger"
)

type IntegrationController struct {
	githubWebhooks *service.GitHubWebhookService
}

func NewIntegrationController(githubWebhooks *service.GitHubWebhookService) *IntegrationController {
	return &IntegrationController{githubWebhooks: githubWebhooks}
}

func (ctl *IntegrationController) GitHubWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid webhook payload", "invalid_payload", nil)
		return
	}

	result, err := ctl.githubWebhooks.Handle(service.GitHubWebhookCommand{
		DeliveryID: c.GetHeader("X-GitHub-Delivery"),
		EventType:  c.GetHeader("X-GitHub-Event"),
		Signature:  c.GetHeader("X-Hub-Signature-256"),
		Payload:    payload,
	})
	if err != nil {
		logGitHubWebhookOutcome(c, c.GetHeader("X-GitHub-Delivery"), c.GetHeader("X-GitHub-Event"), nil, err)
		switch {
		case errors.Is(err, service.ErrWebhookNotConfigured):
			response.Error(c, http.StatusServiceUnavailable, "github webhook not configured", "webhook_not_configured", nil)
		case errors.Is(err, service.ErrInvalidWebhookSignature):
			response.Error(c, http.StatusUnauthorized, "github webhook rejected", "invalid_signature", nil)
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "github webhook rejected", "invalid_payload", nil)
		default:
			response.Error(c, http.StatusInternalServerError, "github webhook failed", "internal_error", nil)
		}
		return
	}

	logGitHubWebhookOutcome(c, result.DeliveryID, result.EventType, result, nil)
	response.JSON(c, http.StatusAccepted, "github webhook accepted", mapper.ToGitHubWebhookResponse(*result))
}

func logGitHubWebhookOutcome(c *gin.Context, deliveryID, eventType string, result *service.GitHubWebhookResult, err error) {
	args := []any{
		"request_id", middleware.GetRequestID(c),
		"correlation_id", middleware.GetCorrelationID(c),
		"delivery_id", deliveryID,
		"event_type", eventType,
	}

	if result != nil {
		args = append(args,
			"status", result.Status,
			"ignored_reason", result.IgnoredReason,
			"trigger_kind", result.Event.TriggerKind,
			"installation_id", result.Event.GitHubInstallationID,
			"repo_full_name", result.Event.RepoFullName,
			"project_id", result.Event.ProjectID,
			"tracked_branch", result.Event.TrackedBranch,
			"commit_sha", result.Event.CommitSHA,
		)
		if result.BuildJob != nil {
			args = append(args,
				"build_job_id", result.BuildJob.ID,
				"build_job_status", result.BuildJob.Status,
			)
		}
	}

	if err != nil {
		args = append(args, "error", err.Error())
		logger.Warn("github_webhook_rejected", args...)
		return
	}
	if result != nil && result.Status == "ignored" {
		logger.Info("github_webhook_ignored", args...)
		return
	}

	logger.Info("github_webhook_accepted", args...)
}
