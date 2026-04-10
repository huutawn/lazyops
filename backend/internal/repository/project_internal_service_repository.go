package repository

import (
	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type ProjectInternalServiceRepository struct {
	db *gorm.DB
}

func NewProjectInternalServiceRepository(db *gorm.DB) *ProjectInternalServiceRepository {
	return &ProjectInternalServiceRepository{db: db}
}

func (r *ProjectInternalServiceRepository) ReplaceForProject(projectID string, items []models.ProjectInternalService) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", projectID).Delete(&models.ProjectInternalService{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

func (r *ProjectInternalServiceRepository) ListByProject(projectID string) ([]models.ProjectInternalService, error) {
	var items []models.ProjectInternalService
	if err := r.db.Where("project_id = ?", projectID).Order("kind ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
