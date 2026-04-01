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

type ProjectStore interface {
	Create(project *models.Project) error
	ListByUser(userID string) ([]models.Project, error)
	GetBySlugForUser(userID, slug string) (*models.Project, error)
	GetByIDForUser(userID, projectID string) (*models.Project, error)
	GetByID(projectID string) (*models.Project, error)
}

type ProjectRepoLinkStore interface {
	Upsert(link *models.ProjectRepoLink) error
	GetByProjectID(projectID string) (*models.ProjectRepoLink, error)
	GetByRepoBranch(githubInstallationID string, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error)
	LookupWebhookRoute(githubInstallationID int64, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error)
}

type AgentStore interface {
	Create(agent *models.Agent) error
	ListByUser(userID string) ([]models.Agent, error)
	GetByAgentIDForUser(userID, agentID string) (*models.Agent, error)
	UpdateStatusForUser(userID, agentID, name, status string, at time.Time) (*models.Agent, error)
}
