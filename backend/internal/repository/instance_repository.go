package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type InstanceRepository struct {
	db *gorm.DB
}

func NewInstanceRepository(db *gorm.DB) *InstanceRepository {
	return &InstanceRepository{db: db}
}

func (r *InstanceRepository) Create(instance *models.Instance) error {
	return r.db.Create(instance).Error
}

func (r *InstanceRepository) ListByUser(userID string) ([]models.Instance, error) {
	var instances []models.Instance
	if err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Order("name ASC").
		Find(&instances).Error; err != nil {
		return nil, err
	}

	return instances, nil
}

func (r *InstanceRepository) GetByNameForUser(userID, name string) (*models.Instance, error) {
	var instance models.Instance
	if err := r.db.Where("user_id = ? AND name = ?", userID, name).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &instance, nil
}

func (r *InstanceRepository) GetByIDForUser(userID, instanceID string) (*models.Instance, error) {
	var instance models.Instance
	if err := r.db.Where("user_id = ? AND id = ?", userID, instanceID).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &instance, nil
}

func (r *InstanceRepository) GetByID(instanceID string) (*models.Instance, error) {
	var instance models.Instance
	if err := r.db.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &instance, nil
}

func (r *InstanceRepository) UpdateAgentState(instanceID, agentID, status string, runtimeCapabilitiesJSON *string, at time.Time) (*models.Instance, error) {
	updates := map[string]any{
		"status":     status,
		"updated_at": at,
	}
	if agentID != "" {
		updates["agent_id"] = agentID
	}
	if runtimeCapabilitiesJSON != nil {
		updates["runtime_capabilities_json"] = *runtimeCapabilitiesJSON
	}

	if err := r.db.Model(&models.Instance{}).
		Where("id = ?", instanceID).
		Updates(updates).Error; err != nil {
		return nil, err
	}

	return r.GetByID(instanceID)
}
