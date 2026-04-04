package repository

import (
	"errors"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type DeploymentBindingRepository struct {
	db *gorm.DB
}

func NewDeploymentBindingRepository(db *gorm.DB) *DeploymentBindingRepository {
	return &DeploymentBindingRepository{db: db}
}

func (r *DeploymentBindingRepository) Create(binding *models.DeploymentBinding) error {
	return r.db.Create(binding).Error
}

func (r *DeploymentBindingRepository) ListByProject(projectID string) ([]models.DeploymentBinding, error) {
	var items []models.DeploymentBinding
	if err := r.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *DeploymentBindingRepository) GetByTargetRefForProject(projectID, targetRef string) (*models.DeploymentBinding, error) {
	var binding models.DeploymentBinding
	if err := r.db.Where("project_id = ? AND target_ref = ?", projectID, targetRef).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &binding, nil
}
