package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
)

type ProjectRepoLinkRepository struct {
	db *gorm.DB
}

func NewProjectRepoLinkRepository(db *gorm.DB) *ProjectRepoLinkRepository {
	return &ProjectRepoLinkRepository{db: db}
}

func (r *ProjectRepoLinkRepository) Upsert(link *models.ProjectRepoLink) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "project_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"github_installation_id": link.GitHubInstallationID,
			"github_repo_id":         link.GitHubRepoID,
			"repo_owner":             link.RepoOwner,
			"repo_name":              link.RepoName,
			"tracked_branch":         link.TrackedBranch,
			"preview_enabled":        link.PreviewEnabled,
			"updated_at":             time.Now().UTC(),
		}),
	}).Create(link).Error
}

func (r *ProjectRepoLinkRepository) GetByProjectID(projectID string) (*models.ProjectRepoLink, error) {
	var link models.ProjectRepoLink
	if err := r.db.Where("project_id = ?", projectID).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &link, nil
}

func (r *ProjectRepoLinkRepository) GetByRepoBranch(githubInstallationID string, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	var link models.ProjectRepoLink
	if err := r.db.
		Where("github_installation_id = ? AND github_repo_id = ? AND tracked_branch = ?", githubInstallationID, githubRepoID, trackedBranch).
		First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &link, nil
}

func (r *ProjectRepoLinkRepository) LookupWebhookRoute(githubInstallationID int64, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	var lookup models.ProjectRepoLink
	if err := r.db.
		Table("project_repo_links").
		Select("project_repo_links.*").
		Joins("JOIN github_installations ON github_installations.id = project_repo_links.github_installation_id").
		Where("github_installations.github_installation_id = ?", githubInstallationID).
		Where("github_installations.revoked_at IS NULL").
		Where("project_repo_links.github_repo_id = ?", githubRepoID).
		Where("project_repo_links.tracked_branch = ?", trackedBranch).
		First(&lookup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &lookup, nil
}
