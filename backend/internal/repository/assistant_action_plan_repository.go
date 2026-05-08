package repository

import (
	"errors"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type AssistantActionPlanRepository struct {
	db *gorm.DB
}

func NewAssistantActionPlanRepository(db *gorm.DB) *AssistantActionPlanRepository {
	return &AssistantActionPlanRepository{db: db}
}

func (r *AssistantActionPlanRepository) Create(item *models.AssistantActionPlan) error {
	return r.db.Create(item).Error
}

func (r *AssistantActionPlanRepository) GetByID(planID string) (*models.AssistantActionPlan, error) {
	var item models.AssistantActionPlan
	if err := r.db.Where("id = ?", planID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *AssistantActionPlanRepository) GetLatestPendingBySession(sessionID string) (*models.AssistantActionPlan, error) {
	var item models.AssistantActionPlan
	if err := r.db.Where("session_id = ? AND status IN ?", sessionID, []string{"draft", "awaiting_confirmation", "executing"}).Order("created_at DESC").First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *AssistantActionPlanRepository) Update(item *models.AssistantActionPlan) error {
	return r.db.Save(item).Error
}
