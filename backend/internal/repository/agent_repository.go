package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type AgentRepository struct {
	db *gorm.DB
}

func NewAgentRepository(db *gorm.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

func (r *AgentRepository) Create(agent *models.Agent) error {
	return r.db.Create(agent).Error
}

func (r *AgentRepository) ListByUser(userID string) ([]models.Agent, error) {
	var agents []models.Agent
	if err := r.db.Where("user_id = ?", userID).Order("updated_at desc").Find(&agents).Error; err != nil {
		return nil, err
	}

	return agents, nil
}

func (r *AgentRepository) GetByAgentIDForUser(userID, agentID string) (*models.Agent, error) {
	var agent models.Agent
	if err := r.db.Where("user_id = ? AND agent_id = ?", userID, agentID).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &agent, nil
}

func (r *AgentRepository) UpdateStatusForUser(userID, agentID, name, status string, at time.Time) (*models.Agent, error) {
	agent, err := r.GetByAgentIDForUser(userID, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, nil
	}

	if name != "" {
		agent.Name = name
	}
	agent.Status = status
	agent.LastSeenAt = &at
	if err := r.db.Save(agent).Error; err != nil {
		return nil, err
	}

	return agent, nil
}
