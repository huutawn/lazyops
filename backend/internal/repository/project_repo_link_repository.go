package repository

import (
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
			"git_hub_installation_id": link.GitHubInstallationID,
			"git_hub_repo_id":         link.GitHubRepoID,
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
	tx := r.db.Where("project_id = ?", projectID).Limit(1).Find(&link)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}

	return &link, nil
}

func (r *ProjectRepoLinkRepository) GetByRepoBranch(githubInstallationID string, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	var link models.ProjectRepoLink
	tx := r.db.
		Where("git_hub_installation_id = ? AND git_hub_repo_id = ? AND tracked_branch = ?", githubInstallationID, githubRepoID, trackedBranch).
		Limit(1).
		Find(&link)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}

	return &link, nil
}

func (r *ProjectRepoLinkRepository) LookupWebhookRoute(githubInstallationID int64, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	var lookup models.ProjectRepoLink

	installationIDs := r.db.
		Model(&models.GitHubInstallation{}).
		Select("id").
		Where("github_installation_id = ?", githubInstallationID).
		Where("revoked_at IS NULL")

	tx := r.db.
		Model(&models.ProjectRepoLink{}).
		Where("project_repo_links.git_hub_installation_id IN (?)", installationIDs).
		Where("project_repo_links.git_hub_repo_id = ?", githubRepoID).
		Where("project_repo_links.tracked_branch = ?", trackedBranch).
		Limit(1).
		Find(&lookup)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}

	return &lookup, nil
}
