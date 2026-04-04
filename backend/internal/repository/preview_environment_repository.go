package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type PreviewEnvironmentRepository struct {
	db *gorm.DB
}

func NewPreviewEnvironmentRepository(db *gorm.DB) *PreviewEnvironmentRepository {
	return &PreviewEnvironmentRepository{db: db}
}

func (r *PreviewEnvironmentRepository) Create(preview *models.PreviewEnvironment) error {
	return r.db.Create(preview).Error
}

func (r *PreviewEnvironmentRepository) GetByIDForProject(projectID, previewID string) (*models.PreviewEnvironment, error) {
	var preview models.PreviewEnvironment
	if err := r.db.Where("project_id = ? AND id = ?", projectID, previewID).First(&preview).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &preview, nil
}

func (r *PreviewEnvironmentRepository) GetByPRNumber(projectID string, prNumber int) (*models.PreviewEnvironment, error) {
	var preview models.PreviewEnvironment
	if err := r.db.Where("project_id = ? AND pr_number = ?", projectID, prNumber).First(&preview).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &preview, nil
}

func (r *PreviewEnvironmentRepository) ListByProject(projectID string) ([]models.PreviewEnvironment, error) {
	var previews []models.PreviewEnvironment
	if err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&previews).Error; err != nil {
		return nil, err
	}
	return previews, nil
}

func (r *PreviewEnvironmentRepository) ListActiveByProject(projectID string) ([]models.PreviewEnvironment, error) {
	var previews []models.PreviewEnvironment
	if err := r.db.Where("project_id = ? AND status NOT IN (?)", projectID, []string{"destroyed", "failed"}).Order("created_at ASC").Find(&previews).Error; err != nil {
		return nil, err
	}
	return previews, nil
}

func (r *PreviewEnvironmentRepository) UpdateStatus(previewID, status string, at time.Time) error {
	return r.db.Model(&models.PreviewEnvironment{}).
		Where("id = ?", previewID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": at,
		}).Error
}

func (r *PreviewEnvironmentRepository) Destroy(previewID, reason string, at time.Time) error {
	return r.db.Model(&models.PreviewEnvironment{}).
		Where("id = ?", previewID).
		Updates(map[string]any{
			"status":         "destroyed",
			"destroy_reason": reason,
			"destroyed_at":   at,
			"updated_at":     at,
		}).Error
}
