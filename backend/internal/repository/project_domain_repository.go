package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type ProjectDomainRepository struct {
	db *gorm.DB
}

func NewProjectDomainRepository(db *gorm.DB) *ProjectDomainRepository {
	return &ProjectDomainRepository{db: db}
}

func (r *ProjectDomainRepository) Create(item *models.ProjectDomain) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("create project domain: %w", err)
	}
	return nil
}

func (r *ProjectDomainRepository) Save(item *models.ProjectDomain) error {
	if err := r.db.Save(item).Error; err != nil {
		return fmt.Errorf("save project domain: %w", err)
	}
	return nil
}

func (r *ProjectDomainRepository) GetByProjectIDAndKind(projectID, kind string) (*models.ProjectDomain, error) {
	var item models.ProjectDomain
	if err := r.db.Where("project_id = ? AND kind = ?", projectID, kind).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("query project domain by project and kind: %w", err)
	}
	return &item, nil
}

func (r *ProjectDomainRepository) GetByHostname(hostname string) (*models.ProjectDomain, error) {
	var item models.ProjectDomain
	if err := r.db.Where("hostname = ?", hostname).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("query project domain by hostname: %w", err)
	}
	return &item, nil
}
