package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
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
