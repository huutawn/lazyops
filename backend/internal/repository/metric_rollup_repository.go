package repository

import (
	"time"

	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type MetricRollupRepository struct {
	db *gorm.DB
}

func NewMetricRollupRepository(db *gorm.DB) *MetricRollupRepository {
	return &MetricRollupRepository{db: db}
}

func (r *MetricRollupRepository) Create(rollup *models.MetricRollup) error {
	return r.db.Create(rollup).Error
}

func (r *MetricRollupRepository) ListByProjectAndService(projectID, serviceName string, windowStart, windowEnd time.Time) ([]models.MetricRollup, error) {
	tx := r.db.Model(&models.MetricRollup{}).Where("project_id = ?", projectID)
	if serviceName != "" {
		tx = tx.Where("service_name = ?", serviceName)
	}
	if !windowStart.IsZero() {
		tx = tx.Where("window_end >= ?", windowStart)
	}
	if !windowEnd.IsZero() {
		tx = tx.Where("window_start <= ?", windowEnd)
	}

	var rollups []models.MetricRollup
	if err := tx.Order("window_end DESC, id DESC").Find(&rollups).Error; err != nil {
		return nil, err
	}
	return rollups, nil
}

func (r *MetricRollupRepository) ListByProject(projectID string, limit int) ([]models.MetricRollup, error) {
	if limit <= 0 {
		limit = 100
	}

	var rollups []models.MetricRollup
	if err := r.db.
		Where("project_id = ?", projectID).
		Order("window_end DESC, id DESC").
		Limit(limit).
		Find(&rollups).Error; err != nil {
		return nil, err
	}
	return rollups, nil
}
