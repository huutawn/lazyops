package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
)

type GitHubInstallationRepository struct {
	db *gorm.DB
}

func NewGitHubInstallationRepository(db *gorm.DB) *GitHubInstallationRepository {
	return &GitHubInstallationRepository{db: db}
}

func (r *GitHubInstallationRepository) Upsert(installation *models.GitHubInstallation) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "github_installation_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"account_login": installation.AccountLogin,
			"account_type":  installation.AccountType,
			"scope_json":    installation.ScopeJSON,
			"installed_at":  installation.InstalledAt,
			"revoked_at":    installation.RevokedAt,
			"updated_at":    time.Now().UTC(),
		}),
	}).Create(installation).Error
}

func (r *GitHubInstallationRepository) ListByUser(userID string) ([]models.GitHubInstallation, error) {
	var installations []models.GitHubInstallation
	if err := r.db.
		Where("user_id = ?", userID).
		Order("revoked_at IS NULL DESC").
		Order("account_login ASC").
		Order("github_installation_id ASC").
		Find(&installations).Error; err != nil {
		return nil, err
	}

	return installations, nil
}

func (r *GitHubInstallationRepository) GetByInstallationIDForUser(userID string, installationID int64) (*models.GitHubInstallation, error) {
	var installation models.GitHubInstallation
	if err := r.db.Where("user_id = ? AND github_installation_id = ?", userID, installationID).First(&installation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &installation, nil
}

func (r *GitHubInstallationRepository) RevokeMissing(userID string, activeInstallationIDs []int64, at time.Time) error {
	query := r.db.Model(&models.GitHubInstallation{}).Where("user_id = ? AND revoked_at IS NULL", userID)
	if len(activeInstallationIDs) > 0 {
		query = query.Where("github_installation_id NOT IN ?", activeInstallationIDs)
	}

	return query.Updates(map[string]any{
		"revoked_at": at,
		"updated_at": at,
	}).Error
}
