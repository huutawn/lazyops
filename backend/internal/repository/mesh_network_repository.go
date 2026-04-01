package repository

import (
	"errors"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type MeshNetworkRepository struct {
	db *gorm.DB
}

func NewMeshNetworkRepository(db *gorm.DB) *MeshNetworkRepository {
	return &MeshNetworkRepository{db: db}
}

func (r *MeshNetworkRepository) Create(mesh *models.MeshNetwork) error {
	return r.db.Create(mesh).Error
}

func (r *MeshNetworkRepository) ListByUser(userID string) ([]models.MeshNetwork, error) {
	var items []models.MeshNetwork
	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Order("name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *MeshNetworkRepository) GetByNameForUser(userID, name string) (*models.MeshNetwork, error) {
	var mesh models.MeshNetwork
	if err := r.db.Where("user_id = ? AND name = ?", userID, name).First(&mesh).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &mesh, nil
}

func (r *MeshNetworkRepository) GetByIDForUser(userID, meshID string) (*models.MeshNetwork, error) {
	var mesh models.MeshNetwork
	if err := r.db.Where("user_id = ? AND id = ?", userID, meshID).First(&mesh).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &mesh, nil
}

func (r *MeshNetworkRepository) GetByID(meshID string) (*models.MeshNetwork, error) {
	var mesh models.MeshNetwork
	if err := r.db.Where("id = ?", meshID).First(&mesh).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &mesh, nil
}
