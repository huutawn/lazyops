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

func (r *AgentRepository) List() ([]models.Agent, error) {
	var agents []models.Agent
	if err := r.db.Order("updated_at desc").Find(&agents).Error; err != nil {
		return nil, err
	}

	return agents, nil
}

func (r *AgentRepository) GetByAgentID(agentID string) (*models.Agent, error) {
	var agent models.Agent
	if err := r.db.Where("agent_id = ?", agentID).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &agent, nil
}

func (r *AgentRepository) UpsertStatus(agentID, name, status string) (*models.Agent, error) {
	agent, err := r.GetByAgentID(agentID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if agent == nil {
		agent = &models.Agent{
			AgentID:    agentID,
			Name:       name,
			Status:     status,
			LastSeenAt: &now,
		}
		return agent, r.db.Create(agent).Error
	}

	agent.Name = name
	agent.Status = status
	agent.LastSeenAt = &now
	if err := r.db.Save(agent).Error; err != nil {
		return nil, err
	}

	return agent, nil
}
