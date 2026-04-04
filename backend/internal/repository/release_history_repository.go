package repository

import (
	"errors"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type ReleaseHistoryRepository struct {
	db *gorm.DB
}

func NewReleaseHistoryRepository(db *gorm.DB) *ReleaseHistoryRepository {
	return &ReleaseHistoryRepository{db: db}
}

func (r *ReleaseHistoryRepository) Create(record *models.ReleaseHistory) error {
	return r.db.Create(record).Error
}

func (r *ReleaseHistoryRepository) ListByProject(projectID string, limit int) ([]models.ReleaseHistory, error) {
	var records []models.ReleaseHistory
	if err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r *ReleaseHistoryRepository) GetByIDForProject(projectID, recordID string) (*models.ReleaseHistory, error) {
	var record models.ReleaseHistory
	if err := r.db.Where("project_id = ? AND id = ?", projectID, recordID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}
