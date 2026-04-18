package repository

import (
	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type ServiceRepository struct {
	db *gorm.DB
}

func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func (r *ServiceRepository) ReplaceForProject(projectID string, items []models.Service) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", projectID).Delete(&models.Service{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

func (r *ServiceRepository) ListByProject(projectID string) ([]models.Service, error) {
	var items []models.Service
	if err := r.db.Where("project_id = ?", projectID).Order("name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
