package mapper

import (
	"strings"

	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/config"
	"lazyops-server/internal/service"
)

func ToSyncGitHubInstallationsCommand(userID string, req requestdto.SyncGitHubInstallationsRequest) service.SyncGitHubInstallationsCommand {
	return service.SyncGitHubInstallationsCommand{
		UserID:            userID,
		GitHubAccessToken: req.GitHubAccessToken,
	}
}

func ToGitHubInstallationSyncResponse(result service.GitHubInstallationSyncResult) responsedto.GitHubInstallationSyncResponse {
	items := make([]responsedto.GitHubInstallationResponse, 0, len(result.Items))
	for _, item := range result.Items {
		repositories := make([]responsedto.GitHubInstallationRepositoryScopeResponse, 0, len(item.Scope.Repositories))
		for _, repository := range item.Scope.Repositories {
			repositories = append(repositories, responsedto.GitHubInstallationRepositoryScopeResponse{
				ID:         repository.ID,
				Name:       repository.Name,
				FullName:   repository.FullName,
				OwnerLogin: repository.OwnerLogin,
				Private:    repository.Private,
			})
		}

		items = append(items, responsedto.GitHubInstallationResponse{
			ID:                   item.ID,
			GitHubInstallationID: item.GitHubInstallationID,
			AccountLogin:         item.AccountLogin,
			AccountType:          item.AccountType,
			InstalledAt:          item.InstalledAt,
			RevokedAt:            item.RevokedAt,
			Status:               item.Status,
			Scope: responsedto.GitHubInstallationScopeResponse{
				RepositorySelection: item.Scope.RepositorySelection,
				Permissions:         item.Scope.Permissions,
				Repositories:        repositories,
			},
		})
	}

	return responsedto.GitHubInstallationSyncResponse{Items: items}
}

func ToGitHubRepositoryListResponse(result service.GitHubRepositoryListResult) responsedto.GitHubRepositoryListResponse {
	items := make([]responsedto.GitHubRepositoryResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, responsedto.GitHubRepositoryResponse{
			GitHubInstallationID:     item.GitHubInstallationID,
			InstallationAccountLogin: item.InstallationAccountLogin,
			InstallationAccountType:  item.InstallationAccountType,
			GitHubRepoID:             item.GitHubRepoID,
			RepoOwner:                item.RepoOwner,
			RepoName:                 item.RepoName,
			FullName:                 item.FullName,
			Private:                  item.Private,
			Permissions:              item.Permissions,
		})
	}

	return responsedto.GitHubRepositoryListResponse{Items: items}
}

func ToGitHubAppConfigResponse(cfg config.GitHubAppConfig) responsedto.GitHubAppConfigResponse {
	installURL := strings.TrimSpace(cfg.InstallURL)
	callbackURL := strings.TrimSpace(cfg.CallbackURL)
	webhookURL := strings.TrimSpace(cfg.WebhookURL)

	return responsedto.GitHubAppConfigResponse{
		Name:        strings.TrimSpace(cfg.Name),
		InstallURL:  installURL,
		WebhookURL:  webhookURL,
		CallbackURL: callbackURL,
		Enabled:     installURL != "" && callbackURL != "" && webhookURL != "",
	}
}

func ToGitHubWebhookResponse(result service.GitHubWebhookResult) responsedto.GitHubWebhookResponse {
	return responsedto.GitHubWebhookResponse{
		DeliveryID:    result.DeliveryID,
		EventType:     result.EventType,
		Status:        result.Status,
		IgnoredReason: result.IgnoredReason,
		Event: responsedto.GitHubWebhookNormalizedEventResponse{
			TriggerKind:          result.Event.TriggerKind,
			Action:               result.Event.Action,
			ProjectID:            result.Event.ProjectID,
			ProjectRepoLinkID:    result.Event.ProjectRepoLinkID,
			GitHubInstallationID: result.Event.GitHubInstallationID,
			GitHubRepoID:         result.Event.GitHubRepoID,
			RepoOwner:            result.Event.RepoOwner,
			RepoName:             result.Event.RepoName,
			RepoFullName:         result.Event.RepoFullName,
			TrackedBranch:        result.Event.TrackedBranch,
			CommitSHA:            result.Event.CommitSHA,
			PullRequestNumber:    result.Event.PullRequestNumber,
			PreviewEnabled:       result.Event.PreviewEnabled,
			ShouldEnqueueBuild:   result.Event.ShouldEnqueueBuild,
			ShouldDestroyPreview: result.Event.ShouldDestroyPreview,
		},
	}
}
