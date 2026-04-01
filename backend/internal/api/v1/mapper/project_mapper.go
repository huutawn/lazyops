package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateProjectCommand(userID string, req requestdto.CreateProjectRequest) service.CreateProjectCommand {
	return service.CreateProjectCommand{
		UserID:        userID,
		Name:          req.Name,
		Slug:          req.Slug,
		DefaultBranch: req.DefaultBranch,
	}
}

func ToCreateProjectRepoLinkCommand(userID, role, projectID string, req requestdto.LinkProjectRepoRequest) service.CreateProjectRepoLinkCommand {
	return service.CreateProjectRepoLinkCommand{
		RequesterUserID:      userID,
		RequesterRole:        role,
		ProjectID:            projectID,
		GitHubInstallationID: req.GitHubInstallationID,
		GitHubRepoID:         req.GitHubRepoID,
		TrackedBranch:        req.TrackedBranch,
		PreviewEnabled:       req.PreviewEnabled,
	}
}

func ToProjectSummaryResponse(summary service.ProjectSummary) responsedto.ProjectSummaryResponse {
	return responsedto.ProjectSummaryResponse{
		ID:            summary.ID,
		Name:          summary.Name,
		Slug:          summary.Slug,
		DefaultBranch: summary.DefaultBranch,
		CreatedAt:     summary.CreatedAt,
		UpdatedAt:     summary.UpdatedAt,
	}
}

func ToProjectListResponse(items []service.ProjectSummary) responsedto.ProjectListResponse {
	responseItems := make([]responsedto.ProjectSummaryResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, ToProjectSummaryResponse(item))
	}

	return responsedto.ProjectListResponse{Items: responseItems}
}

func ToProjectRepoLinkResponse(record service.ProjectRepoLinkRecord) responsedto.ProjectRepoLinkResponse {
	return responsedto.ProjectRepoLinkResponse{
		ID:                   record.ID,
		ProjectID:            record.ProjectID,
		GitHubInstallationID: record.GitHubInstallationID,
		GitHubRepoID:         record.GitHubRepoID,
		RepoOwner:            record.RepoOwner,
		RepoName:             record.RepoName,
		RepoFullName:         record.RepoFullName,
		TrackedBranch:        record.TrackedBranch,
		PreviewEnabled:       record.PreviewEnabled,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
	}
}
