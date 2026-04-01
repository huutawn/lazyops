package service

import (
	"errors"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var (
	ErrProjectNotFound      = errors.New("project not found")
	ErrProjectAccessDenied  = errors.New("project access denied")
	ErrRepoNotAccessible    = errors.New("repo not accessible")
	ErrInvalidTrackedBranch = errors.New("invalid tracked branch")
	ErrRepoLinkConflict     = errors.New("repo link conflict")
	ErrRepoLinkNotFound     = errors.New("repo link not found")
)

type ProjectRepoLinkService struct {
	projects      ProjectStore
	installations GitHubInstallationStore
	repoLinks     ProjectRepoLinkStore
}

func NewProjectRepoLinkService(
	projects ProjectStore,
	installations GitHubInstallationStore,
	repoLinks ProjectRepoLinkStore,
) *ProjectRepoLinkService {
	return &ProjectRepoLinkService{
		projects:      projects,
		installations: installations,
		repoLinks:     repoLinks,
	}
}

func (s *ProjectRepoLinkService) LinkRepository(cmd CreateProjectRepoLinkCommand) (*ProjectRepoLinkRecord, error) {
	project, err := s.resolveProjectForWrite(cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}

	if cmd.GitHubInstallationID <= 0 || cmd.GitHubRepoID <= 0 {
		return nil, ErrInvalidInput
	}

	trackedBranch, err := normalizeTrackedBranch(cmd.TrackedBranch, project.DefaultBranch)
	if err != nil {
		return nil, err
	}

	installation, repository, err := s.resolveInstallationRepository(project.UserID, cmd.GitHubInstallationID, cmd.GitHubRepoID)
	if err != nil {
		return nil, err
	}

	conflicting, err := s.repoLinks.GetByRepoBranch(installation.ID, cmd.GitHubRepoID, trackedBranch)
	if err != nil {
		return nil, err
	}
	if conflicting != nil && conflicting.ProjectID != project.ID {
		return nil, ErrRepoLinkConflict
	}

	existing, err := s.repoLinks.GetByProjectID(project.ID)
	if err != nil {
		return nil, err
	}

	link := &models.ProjectRepoLink{
		ID:                   utils.NewPrefixedID("prl"),
		ProjectID:            project.ID,
		GitHubInstallationID: installation.ID,
		GitHubRepoID:         cmd.GitHubRepoID,
		RepoOwner:            repository.OwnerLogin,
		RepoName:             repository.Name,
		TrackedBranch:        trackedBranch,
		PreviewEnabled:       cmd.PreviewEnabled,
	}
	if existing != nil {
		link.ID = existing.ID
		link.CreatedAt = existing.CreatedAt
	}

	if err := s.repoLinks.Upsert(link); err != nil {
		return nil, err
	}

	persisted, err := s.repoLinks.GetByProjectID(project.ID)
	if err != nil {
		return nil, err
	}
	if persisted == nil {
		return nil, ErrRepoLinkNotFound
	}

	record := ToProjectRepoLinkRecord(*persisted, installation.GitHubInstallationID)
	return &record, nil
}

func (s *ProjectRepoLinkService) LookupWebhookRoute(cmd WebhookRouteLookupCommand) (*ProjectRepoLinkRecord, error) {
	if cmd.GitHubInstallationID <= 0 || cmd.GitHubRepoID <= 0 {
		return nil, ErrInvalidInput
	}

	trackedBranch, err := normalizeTrackedBranch(cmd.TrackedBranch, "")
	if err != nil {
		return nil, err
	}

	link, err := s.repoLinks.LookupWebhookRoute(cmd.GitHubInstallationID, cmd.GitHubRepoID, trackedBranch)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, ErrRepoLinkNotFound
	}

	record := ToProjectRepoLinkRecord(*link, cmd.GitHubInstallationID)
	return &record, nil
}

func ToProjectRepoLinkRecord(link models.ProjectRepoLink, githubInstallationID int64) ProjectRepoLinkRecord {
	return ProjectRepoLinkRecord{
		ID:                         link.ID,
		ProjectID:                  link.ProjectID,
		GitHubInstallationRecordID: link.GitHubInstallationID,
		GitHubInstallationID:       githubInstallationID,
		GitHubRepoID:               link.GitHubRepoID,
		RepoOwner:                  link.RepoOwner,
		RepoName:                   link.RepoName,
		RepoFullName:               link.RepoOwner + "/" + link.RepoName,
		TrackedBranch:              link.TrackedBranch,
		PreviewEnabled:             link.PreviewEnabled,
		CreatedAt:                  link.CreatedAt,
		UpdatedAt:                  link.UpdatedAt,
	}
}

func (s *ProjectRepoLinkService) resolveProjectForWrite(requesterUserID, requesterRole, projectID string) (*models.Project, error) {
	requesterUserID = strings.TrimSpace(requesterUserID)
	projectID = strings.TrimSpace(projectID)
	if requesterUserID == "" || projectID == "" {
		return nil, ErrInvalidInput
	}

	if requesterRole == RoleAdmin {
		project, err := s.projects.GetByID(projectID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, ErrProjectNotFound
		}
		return project, nil
	}

	project, err := s.projects.GetByIDForUser(requesterUserID, projectID)
	if err != nil {
		return nil, err
	}
	if project != nil {
		return project, nil
	}

	anyProject, err := s.projects.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if anyProject == nil {
		return nil, ErrProjectNotFound
	}

	return nil, ErrProjectAccessDenied
}

func (s *ProjectRepoLinkService) resolveInstallationRepository(ownerUserID string, githubInstallationID int64, githubRepoID int64) (*models.GitHubInstallation, *GitHubInstallationRepositoryScope, error) {
	installation, err := s.installations.GetByInstallationIDForUser(ownerUserID, githubInstallationID)
	if err != nil {
		return nil, nil, err
	}
	if installation == nil || installation.RevokedAt != nil {
		return nil, nil, ErrRepoNotAccessible
	}

	scope, err := parseGitHubInstallationScope(installation.ScopeJSON)
	if err != nil {
		return nil, nil, err
	}

	for _, repository := range scope.Repositories {
		if repository.ID == githubRepoID {
			repository := repository
			return installation, &repository, nil
		}
	}

	return nil, nil, ErrRepoNotAccessible
}

func normalizeTrackedBranch(branch string, fallback string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = strings.TrimSpace(fallback)
	}
	if branch == "" {
		return "", ErrInvalidTrackedBranch
	}
	if strings.HasPrefix(branch, "refs/") && !strings.HasPrefix(branch, "refs/heads/") {
		return "", ErrInvalidTrackedBranch
	}
	branch = strings.TrimPrefix(branch, "refs/heads/")
	if branch == "" || strings.ContainsAny(branch, " \t\r\n") {
		return "", ErrInvalidTrackedBranch
	}

	return branch, nil
}
