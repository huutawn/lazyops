package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type BootstrapTokenRepository struct {
	db *gorm.DB
}

func NewBootstrapTokenRepository(db *gorm.DB) *BootstrapTokenRepository {
	return &BootstrapTokenRepository{db: db}
}

func (r *BootstrapTokenRepository) Create(token *models.BootstrapToken) error {
	return r.db.Create(token).Error
}

func (r *BootstrapTokenRepository) GetByHash(tokenHash string) (*models.BootstrapToken, error) {
	var token models.BootstrapToken
	if err := r.db.Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &token, nil
}

func (r *BootstrapTokenRepository) MarkUsed(tokenID string, at time.Time) error {
	return r.db.Model(&models.BootstrapToken{}).
		Where("id = ?", tokenID).
		Updates(map[string]any{
			"used_at": at,
		}).Error
}
