package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type DeploymentRepository struct {
	db *gorm.DB
}

func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (r *DeploymentRepository) Create(deployment *models.Deployment) error {
	return r.db.Create(deployment).Error
}

func (r *DeploymentRepository) GetByIDForProject(projectID, deploymentID string) (*models.Deployment, error) {
	var deployment models.Deployment
	if err := r.db.Where("project_id = ? AND id = ?", projectID, deploymentID).First(&deployment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &deployment, nil
}

func (r *DeploymentRepository) UpdateStatus(deploymentID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error {
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

	return r.db.Model(&models.Deployment{}).
		Where("id = ?", deploymentID).
		Updates(updates).Error
}
