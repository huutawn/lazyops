package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type GatewayConfigIntentRepository struct {
	db *gorm.DB
}

func NewGatewayConfigIntentRepository(db *gorm.DB) *GatewayConfigIntentRepository {
	return &GatewayConfigIntentRepository{db: db}
}

func (r *GatewayConfigIntentRepository) Create(intent *models.GatewayConfigIntent) error {
	return r.db.Create(intent).Error
}

func (r *GatewayConfigIntentRepository) GetByIDForProject(projectID, intentID string) (*models.GatewayConfigIntent, error) {
	var intent models.GatewayConfigIntent
	if err := r.db.Where("project_id = ? AND id = ?", projectID, intentID).First(&intent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &intent, nil
}

func (r *GatewayConfigIntentRepository) ListByDeployment(projectID, deploymentID string) ([]models.GatewayConfigIntent, error) {
	var intents []models.GatewayConfigIntent
	if err := r.db.Where("project_id = ? AND deployment_id = ?", projectID, deploymentID).Order("created_at DESC").Find(&intents).Error; err != nil {
		return nil, err
	}
	return intents, nil
}

func (r *GatewayConfigIntentRepository) UpdateStatus(intentID, status string, at time.Time) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": at,
	}
	if status == "acknowledged" {
		updates["acknowledged_at"] = at
	}
	if status == "dispatched" {
		updates["dispatched_at"] = at
	}
	return r.db.Model(&models.GatewayConfigIntent{}).
		Where("id = ?", intentID).
		Updates(updates).Error
}
