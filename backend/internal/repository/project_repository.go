package repository

import (
	"errors"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type ProjectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(project *models.Project) error {
	return r.db.Create(project).Error
}

func (r *ProjectRepository) ListByUser(userID string) ([]models.Project, error) {
	var projects []models.Project
	if err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Order("slug ASC").
		Find(&projects).Error; err != nil {
		return nil, err
	}

	return projects, nil
}

func (r *ProjectRepository) GetBySlugForUser(userID, slug string) (*models.Project, error) {
	var project models.Project
	if err := r.db.Where("user_id = ? AND slug = ?", userID, slug).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &project, nil
}

func (r *ProjectRepository) GetByIDForUser(userID, projectID string) (*models.Project, error) {
	var project models.Project
	if err := r.db.Where("user_id = ? AND id = ?", userID, projectID).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &project, nil
}

func (r *ProjectRepository) GetByID(projectID string) (*models.Project, error) {
	var project models.Project
	if err := r.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &project, nil
}
