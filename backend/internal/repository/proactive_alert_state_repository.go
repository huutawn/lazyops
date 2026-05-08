package repository

import (
	"errors"

	"lazyops-server/internal/models"

	"gorm.io/gorm"
)

type ProactiveAlertStateRepository struct {
	db *gorm.DB
}

func NewProactiveAlertStateRepository(db *gorm.DB) *ProactiveAlertStateRepository {
	return &ProactiveAlertStateRepository{db: db}
}

func (r *ProactiveAlertStateRepository) GetByFingerprint(projectID, serviceName, fingerprint string) (*models.ProactiveAlertState, error) {
	var state models.ProactiveAlertState
	if err := r.db.Where("project_id = ? AND service_name = ? AND fingerprint = ?", projectID, serviceName, fingerprint).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (r *ProactiveAlertStateRepository) Upsert(state *models.ProactiveAlertState) error {
	return r.db.Save(state).Error
}
