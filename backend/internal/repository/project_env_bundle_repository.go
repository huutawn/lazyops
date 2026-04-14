package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

type ProjectEnvBundleRepository struct {
	db *gorm.DB
}

func NewProjectEnvBundleRepository(db *gorm.DB) *ProjectEnvBundleRepository {
	return &ProjectEnvBundleRepository{db: db}
}

func (r *ProjectEnvBundleRepository) GetByProject(projectID string) (*models.ProjectEnvBundle, error) {
	var bundle models.ProjectEnvBundle
	err := r.db.Where("project_id = ?", projectID).First(&bundle).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("query project env bundle: %w", err)
	}
	return &bundle, nil
}

func (r *ProjectEnvBundleRepository) Upsert(bundle *models.ProjectEnvBundle) error {
	if bundle.ID == "" {
		bundle.ID = utils.NewPrefixedID("penv")
	}
	now := time.Now().UTC()
	if bundle.CreatedAt.IsZero() {
		bundle.CreatedAt = now
	}
	bundle.UpdatedAt = now

	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"env_encrypted",
			"env_fingerprint",
			"key_names_json",
			"parse_warnings_json",
			"updated_by",
			"updated_at",
		}),
	}).Create(bundle).Error
}

func (r *ProjectEnvBundleRepository) DeleteByProject(projectID string) error {
	return r.db.Where("project_id = ?", projectID).Delete(&models.ProjectEnvBundle{}).Error
}
