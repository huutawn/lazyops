package repository

import (
	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type AssistantMessageRepository struct {
	db *gorm.DB
}

func NewAssistantMessageRepository(db *gorm.DB) *AssistantMessageRepository {
	return &AssistantMessageRepository{db: db}
}

func (r *AssistantMessageRepository) Create(item *models.AssistantMessage) error {
	return r.db.Create(item).Error
}

func (r *AssistantMessageRepository) ListBySession(sessionID string) ([]models.AssistantMessage, error) {
	var items []models.AssistantMessage
	if err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
