package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type AssistantSessionRepository struct {
	db *gorm.DB
}

func NewAssistantSessionRepository(db *gorm.DB) *AssistantSessionRepository {
	return &AssistantSessionRepository{db: db}
}

func (r *AssistantSessionRepository) Create(item *models.AssistantSession) error {
	return r.db.Create(item).Error
}

func (r *AssistantSessionRepository) GetByIDForUser(sessionID, userID string) (*models.AssistantSession, error) {
	var item models.AssistantSession
	if err := r.db.Where("id = ? AND user_id = ?", sessionID, userID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *AssistantSessionRepository) ListByUser(userID string) ([]models.AssistantSession, error) {
	var items []models.AssistantSession
	if err := r.db.Where("user_id = ?", userID).Order("last_message_at DESC, created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *AssistantSessionRepository) Update(item *models.AssistantSession) error {
	return r.db.Save(item).Error
}

func (r *AssistantSessionRepository) Touch(sessionID string, at time.Time) error {
	return r.db.Model(&models.AssistantSession{}).Where("id = ?", sessionID).Updates(map[string]any{
		"last_message_at": at,
		"updated_at":      at,
	}).Error
}
