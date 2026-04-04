package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type RuntimeIncidentRepository struct {
	db *gorm.DB
}

func NewRuntimeIncidentRepository(db *gorm.DB) *RuntimeIncidentRepository {
	return &RuntimeIncidentRepository{db: db}
}

func (r *RuntimeIncidentRepository) Create(incident *models.RuntimeIncident) error {
	return r.db.Create(incident).Error
}

func (r *RuntimeIncidentRepository) ListByProject(projectID string) ([]models.RuntimeIncident, error) {
	var incidents []models.RuntimeIncident
	if err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&incidents).Error; err != nil {
		return nil, err
	}
	return incidents, nil
}

func (r *RuntimeIncidentRepository) ListByDeployment(projectID, deploymentID string) ([]models.RuntimeIncident, error) {
	var incidents []models.RuntimeIncident
	if err := r.db.Where("project_id = ? AND deployment_id = ?", projectID, deploymentID).Order("created_at DESC").Find(&incidents).Error; err != nil {
		return nil, err
	}
	return incidents, nil
}

func (r *RuntimeIncidentRepository) UpdateStatus(incidentID, status string, at time.Time) error {
	return r.db.Model(&models.RuntimeIncident{}).
		Where("id = ?", incidentID).
		Updates(map[string]any{
			"status":      status,
			"resolved_at": at,
			"updated_at":  at,
		}).Error
}

func (r *RuntimeIncidentRepository) GetByIDForProject(projectID, incidentID string) (*models.RuntimeIncident, error) {
	var incident models.RuntimeIncident
	if err := r.db.Where("project_id = ? AND id = ?", projectID, incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &incident, nil
}
