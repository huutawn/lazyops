package repository

import (
	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type LogStreamRepository struct {
	db *gorm.DB
}

func NewLogStreamRepository(db *gorm.DB) *LogStreamRepository {
	return &LogStreamRepository{db: db}
}

func (r *LogStreamRepository) CreateBatch(entries []models.LogStreamEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return r.db.Create(&entries).Error
}

func (r *LogStreamRepository) ListByQuery(query models.LogStreamQuery) ([]models.LogStreamEntry, error) {
	tx := r.db.Model(&models.LogStreamEntry{}).Where("project_id = ?", query.ProjectID)

	if query.ServiceName != "" {
		tx = tx.Where("service_name = ?", query.ServiceName)
	}
	if query.Level != "" {
		tx = tx.Where("level = ?", query.Level)
	}
	if query.Node != "" {
		tx = tx.Where("node = ?", query.Node)
	}
	if query.CorrelationID != "" {
		tx = tx.Where("correlation_id = ?", query.CorrelationID)
	}
	if query.Contains != "" {
		tx = tx.Where("message ILIKE ?", "%"+query.Contains+"%")
	}
	if !query.BeforeOccurredAt.IsZero() && query.BeforeID != "" {
		tx = tx.Where(
			"(occurred_at < ?) OR (occurred_at = ? AND id < ?)",
			query.BeforeOccurredAt,
			query.BeforeOccurredAt,
			query.BeforeID,
		)
	}

	var entries []models.LogStreamEntry
	if err := tx.Order("occurred_at DESC, id DESC").Limit(query.Limit).Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
