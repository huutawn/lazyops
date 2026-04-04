package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type TunnelSessionRepository struct {
	db *gorm.DB
}

func NewTunnelSessionRepository(db *gorm.DB) *TunnelSessionRepository {
	return &TunnelSessionRepository{db: db}
}

func (r *TunnelSessionRepository) Create(session *models.TunnelSession) error {
	return r.db.Create(session).Error
}

func (r *TunnelSessionRepository) GetByID(sessionID string) (*models.TunnelSession, error) {
	var session models.TunnelSession
	if err := r.db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (r *TunnelSessionRepository) ListByTarget(targetKind, targetID string) ([]models.TunnelSession, error) {
	var sessions []models.TunnelSession
	if err := r.db.Where("target_kind = ? AND target_id = ?", targetKind, targetID).Order("created_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *TunnelSessionRepository) CloseSession(sessionID string, at time.Time) error {
	return r.db.Model(&models.TunnelSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{
			"status":     "closed",
			"updated_at": at,
		}).Error
}

func (r *TunnelSessionRepository) CleanupExpired(before time.Time) error {
	return r.db.Model(&models.TunnelSession{}).
		Where("status = ? AND expires_at < ?", "active", before).
		Updates(map[string]any{
			"status":     "expired",
			"updated_at": time.Now().UTC(),
		}).Error
}
