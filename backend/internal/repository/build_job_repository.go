package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
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

func (r *BuildJobRepository) GetByIDForProject(projectID, buildJobID string) (*models.BuildJob, error) {
	var job models.BuildJob
	if err := r.db.Where("project_id = ? AND id = ?", projectID, buildJobID).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
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
