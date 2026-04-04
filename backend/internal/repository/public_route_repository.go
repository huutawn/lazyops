package repository

import (
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type PublicRouteRepository struct {
	db *gorm.DB
}

func NewPublicRouteRepository(db *gorm.DB) *PublicRouteRepository {
	return &PublicRouteRepository{db: db}
}

func (r *PublicRouteRepository) Create(route *models.PublicRoute) error {
	return r.db.Create(route).Error
}

func (r *PublicRouteRepository) ListByProject(projectID string) ([]models.PublicRoute, error) {
	var routes []models.PublicRoute
	if err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

func (r *PublicRouteRepository) ListByDeployment(projectID, deploymentID string) ([]models.PublicRoute, error) {
	var routes []models.PublicRoute
	if err := r.db.Where("project_id = ? AND deployment_id = ?", projectID, deploymentID).Order("created_at DESC").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

func (r *PublicRouteRepository) UpdateStatus(routeID, status string, at time.Time) error {
	return r.db.Model(&models.PublicRoute{}).
		Where("id = ?", routeID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": at,
		}).Error
}
