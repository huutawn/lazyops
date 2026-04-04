package repository

import (
	"errors"

	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type BlueprintRepository struct {
	db *gorm.DB
}

func NewBlueprintRepository(db *gorm.DB) *BlueprintRepository {
	return &BlueprintRepository{db: db}
}

func (r *BlueprintRepository) Create(blueprint *models.Blueprint) error {
	return r.db.Create(blueprint).Error
}

func (r *BlueprintRepository) GetByIDForProject(projectID, blueprintID string) (*models.Blueprint, error) {
	var blueprint models.Blueprint
	if err := r.db.Where("project_id = ? AND id = ?", projectID, blueprintID).First(&blueprint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &blueprint, nil
}

func (r *BlueprintRepository) GetLatestByProject(projectID string) (*models.Blueprint, error) {
	var blueprint models.Blueprint
	if err := r.db.
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		First(&blueprint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &blueprint, nil
}
