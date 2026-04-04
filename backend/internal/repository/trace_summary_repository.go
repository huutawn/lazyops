package repository

import (
	"errors"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type TraceSummaryRepository struct {
	db *gorm.DB
}

func NewTraceSummaryRepository(db *gorm.DB) *TraceSummaryRepository {
	return &TraceSummaryRepository{db: db}
}

func (r *TraceSummaryRepository) Create(trace *models.TraceSummary) error {
	return r.db.Create(trace).Error
}

func (r *TraceSummaryRepository) GetByCorrelationID(correlationID string) (*models.TraceSummary, error) {
	var trace models.TraceSummary
	if err := r.db.Where("correlation_id = ?", correlationID).First(&trace).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &trace, nil
}

func (r *TraceSummaryRepository) ListByProject(projectID string, limit int) ([]models.TraceSummary, error) {
	var traces []models.TraceSummary
	if err := r.db.Where("project_id = ?", projectID).Order("received_at DESC").Limit(limit).Find(&traces).Error; err != nil {
		return nil, err
	}
	return traces, nil
}
