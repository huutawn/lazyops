package repository

import (
	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type AssistantAuditEventRepository struct {
	db *gorm.DB
}

func NewAssistantAuditEventRepository(db *gorm.DB) *AssistantAuditEventRepository {
	return &AssistantAuditEventRepository{db: db}
}

func (r *AssistantAuditEventRepository) Create(item *models.AssistantAuditEvent) error {
	return r.db.Create(item).Error
}
