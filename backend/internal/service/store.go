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

type PATStore interface {
	Create(token *models.PersonalAccessToken) error
	GetByHash(tokenHash string) (*models.PersonalAccessToken, error)
	GetByID(tokenID string) (*models.PersonalAccessToken, error)
	RevokeByIDForUser(userID, tokenID string, at time.Time) error
	TouchLastUsed(tokenID string, at time.Time) error
}

type OAuthIdentityStore interface {
	Create(identity *models.OAuthIdentity) error
	GetByProviderSubject(provider, subject string) (*models.OAuthIdentity, error)
	GetByUserProvider(userID, provider string) (*models.OAuthIdentity, error)
	UpdateProfile(identityID, email, avatarURL string, at time.Time) error
}

type GitHubInstallationStore interface {
	Upsert(installation *models.GitHubInstallation) error
	ListByUser(userID string) ([]models.GitHubInstallation, error)
	GetByInstallationIDForUser(userID string, installationID int64) (*models.GitHubInstallation, error)
	RevokeMissing(userID string, activeInstallationIDs []int64, at time.Time) error
}

type AgentStore interface {
	Create(agent *models.Agent) error
	ListByUser(userID string) ([]models.Agent, error)
	GetByAgentIDForUser(userID, agentID string) (*models.Agent, error)
	UpdateStatusForUser(userID, agentID, name, status string, at time.Time) (*models.Agent, error)
}
