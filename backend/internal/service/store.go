package service

import (
	"time"

	"lazyops-server/internal/models"
)

type UserStore interface {
	Create(user *models.User) error
	GetByEmail(email string) (*models.User, error)
	GetByID(id string) (*models.User, error)
	TouchLastLogin(userID string, at time.Time) error
}

type AgentStore interface {
	Create(agent *models.Agent) error
	ListByUser(userID string) ([]models.Agent, error)
	GetByAgentIDForUser(userID, agentID string) (*models.Agent, error)
	UpdateStatusForUser(userID, agentID, name, status string, at time.Time) (*models.Agent, error)
}
