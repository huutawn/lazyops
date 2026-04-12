package repository

import (
	"time"

	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type BuildJobRepository struct {
	db *gorm.DB
}

func NewBuildJobRepository(db *gorm.DB) *BuildJobRepository {
	return &BuildJobRepository{db: db}
}

func (r *BuildJobRepository) Create(job *models.BuildJob) error {
	return r.db.Create(job).Error
}

func (r *BuildJobRepository) GetByDeliveryID(deliveryID string) (*models.BuildJob, error) {
	var job models.BuildJob
	tx := r.db.Where("github_delivery_id = ?", deliveryID).Order("created_at ASC").Limit(1).Find(&job)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}

	return &job, nil
}

func (r *BuildJobRepository) GetByIDForProject(projectID, buildJobID string) (*models.BuildJob, error) {
	var job models.BuildJob
	tx := r.db.Where("project_id = ? AND id = ?", projectID, buildJobID).Limit(1).Find(&job)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}

	return &job, nil
}

func (r *BuildJobRepository) UpdateStatus(buildJobID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": updatedAt,
	}
	if startedAt != nil {
		updates["started_at"] = *startedAt
	}
	if completedAt != nil {
		updates["completed_at"] = *completedAt
	}

	return r.db.Model(&models.BuildJob{}).
		Where("id = ?", buildJobID).
		Updates(updates).Error
}

func (r *BuildJobRepository) UpdateResult(buildJobID, status, artifactMetadataJSON string, startedAt, completedAt *time.Time, updatedAt time.Time) error {
	updates := map[string]any{
		"status":                 status,
		"artifact_metadata_json": artifactMetadataJSON,
		"updated_at":             updatedAt,
	}
	if startedAt != nil {
		updates["started_at"] = *startedAt
	}
	if completedAt != nil {
		updates["completed_at"] = *completedAt
	}

	return r.db.Model(&models.BuildJob{}).
		Where("id = ?", buildJobID).
		Updates(updates).Error
}
