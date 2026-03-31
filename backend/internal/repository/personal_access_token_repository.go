package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type PersonalAccessTokenRepository struct {
	db *gorm.DB
}

func NewPersonalAccessTokenRepository(db *gorm.DB) *PersonalAccessTokenRepository {
	return &PersonalAccessTokenRepository{db: db}
}

func (r *PersonalAccessTokenRepository) Create(token *models.PersonalAccessToken) error {
	return r.db.Create(token).Error
}

func (r *PersonalAccessTokenRepository) GetByHash(tokenHash string) (*models.PersonalAccessToken, error) {
	var token models.PersonalAccessToken
	if err := r.db.Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *PersonalAccessTokenRepository) GetByID(tokenID string) (*models.PersonalAccessToken, error) {
	var token models.PersonalAccessToken
	if err := r.db.First(&token, "id = ?", tokenID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *PersonalAccessTokenRepository) RevokeByIDForUser(userID, tokenID string, at time.Time) error {
	return r.db.Model(&models.PersonalAccessToken{}).
		Where("id = ? AND user_id = ?", tokenID, userID).
		Updates(map[string]any{
			"revoked_at": at,
			"updated_at": at,
		}).Error
}

func (r *PersonalAccessTokenRepository) TouchLastUsed(tokenID string, at time.Time) error {
	return r.db.Model(&models.PersonalAccessToken{}).
		Where("id = ?", tokenID).
		Updates(map[string]any{
			"last_used_at": at,
			"updated_at":   at,
		}).Error
}
