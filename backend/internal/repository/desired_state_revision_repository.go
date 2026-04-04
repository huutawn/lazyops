package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type DesiredStateRevisionRepository struct {
	db *gorm.DB
}

func NewDesiredStateRevisionRepository(db *gorm.DB) *DesiredStateRevisionRepository {
	return &DesiredStateRevisionRepository{db: db}
}

func (r *DesiredStateRevisionRepository) Create(revision *models.DesiredStateRevision) error {
	return r.db.Create(revision).Error
}

func (r *DesiredStateRevisionRepository) GetByIDForProject(projectID, revisionID string) (*models.DesiredStateRevision, error) {
	var revision models.DesiredStateRevision
	if err := r.db.Where("project_id = ? AND id = ?", projectID, revisionID).First(&revision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &revision, nil
}

func (r *DesiredStateRevisionRepository) UpdateStatus(revisionID, status string, at time.Time) error {
	return r.db.Model(&models.DesiredStateRevision{}).
		Where("id = ?", revisionID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": at,
		}).Error
}
