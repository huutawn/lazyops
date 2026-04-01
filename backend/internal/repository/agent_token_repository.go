package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type AgentTokenRepository struct {
	db *gorm.DB
}

func NewAgentTokenRepository(db *gorm.DB) *AgentTokenRepository {
	return &AgentTokenRepository{db: db}
}

func (r *AgentTokenRepository) Create(token *models.AgentToken) error {
	return r.db.Create(token).Error
}

func (r *AgentTokenRepository) GetByHash(tokenHash string) (*models.AgentToken, error) {
	var token models.AgentToken
	if err := r.db.Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &token, nil
}

func (r *AgentTokenRepository) TouchLastUsed(tokenID string, at time.Time) error {
	return r.db.Model(&models.AgentToken{}).
		Where("id = ?", tokenID).
		Updates(map[string]any{"last_used_at": at}).Error
}

func (r *AgentTokenRepository) RevokeByAgent(agentID string, at time.Time) error {
	return r.db.Model(&models.AgentToken{}).
		Where("agent_id = ? AND revoked_at IS NULL", agentID).
		Updates(map[string]any{"revoked_at": at}).Error
}
