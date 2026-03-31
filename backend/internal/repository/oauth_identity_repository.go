package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type OAuthIdentityRepository struct {
	db *gorm.DB
}

func NewOAuthIdentityRepository(db *gorm.DB) *OAuthIdentityRepository {
	return &OAuthIdentityRepository{db: db}
}

func (r *OAuthIdentityRepository) Create(identity *models.OAuthIdentity) error {
	return r.db.Create(identity).Error
}

func (r *OAuthIdentityRepository) GetByProviderSubject(provider, subject string) (*models.OAuthIdentity, error) {
	var identity models.OAuthIdentity
	if err := r.db.Where("provider = ? AND provider_subject = ?", provider, subject).First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &identity, nil
}

func (r *OAuthIdentityRepository) UpdateProfile(identityID, email, avatarURL string, at time.Time) error {
	return r.db.Model(&models.OAuthIdentity{}).
		Where("id = ?", identityID).
		Updates(map[string]any{
			"email":      email,
			"avatar_url": avatarURL,
			"updated_at": at,
		}).Error
}
