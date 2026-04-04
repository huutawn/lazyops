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

type BuildJobStore interface {
	Create(job *models.BuildJob) error
	GetByIDForProject(projectID, buildJobID string) (*models.BuildJob, error)
	UpdateStatus(buildJobID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error
}

type DeploymentBindingStore interface {
	Create(binding *models.DeploymentBinding) error
	ListByProject(projectID string) ([]models.DeploymentBinding, error)
	GetByTargetRefForProject(projectID, targetRef string) (*models.DeploymentBinding, error)
}

type ProjectServiceStore interface {
	ReplaceForProject(projectID string, items []models.Service) error
	ListByProject(projectID string) ([]models.Service, error)
}

type BlueprintStore interface {
	Create(blueprint *models.Blueprint) error
	GetByIDForProject(projectID, blueprintID string) (*models.Blueprint, error)
}

type DesiredStateRevisionStore interface {
	Create(revision *models.DesiredStateRevision) error
	GetByIDForProject(projectID, revisionID string) (*models.DesiredStateRevision, error)
	UpdateStatus(revisionID, status string, at time.Time) error
}

type DeploymentStore interface {
	Create(deployment *models.Deployment) error
	GetByIDForProject(projectID, deploymentID string) (*models.Deployment, error)
	UpdateStatus(deploymentID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error
}

type InstanceStore interface {
	Create(instance *models.Instance) error
	ListByUser(userID string) ([]models.Instance, error)
	GetByNameForUser(userID, name string) (*models.Instance, error)
	GetByIDForUser(userID, instanceID string) (*models.Instance, error)
	GetByID(instanceID string) (*models.Instance, error)
	UpdateAgentState(instanceID, agentID, status string, runtimeCapabilitiesJSON *string, at time.Time) (*models.Instance, error)
}

type MeshNetworkStore interface {
	Create(mesh *models.MeshNetwork) error
	ListByUser(userID string) ([]models.MeshNetwork, error)
	GetByNameForUser(userID, name string) (*models.MeshNetwork, error)
	GetByIDForUser(userID, meshID string) (*models.MeshNetwork, error)
	GetByID(meshID string) (*models.MeshNetwork, error)
}

type ClusterStore interface {
	Create(cluster *models.Cluster) error
	ListByUser(userID string) ([]models.Cluster, error)
	GetByNameForUser(userID, name string) (*models.Cluster, error)
	GetByIDForUser(userID, clusterID string) (*models.Cluster, error)
	GetByID(clusterID string) (*models.Cluster, error)
}

type BootstrapTokenStore interface {
	Create(token *models.BootstrapToken) error
	GetByHash(tokenHash string) (*models.BootstrapToken, error)
	MarkUsed(tokenID string, at time.Time) error
}

type AgentTokenStore interface {
	Create(token *models.AgentToken) error
	GetByHash(tokenHash string) (*models.AgentToken, error)
	TouchLastUsed(tokenID string, at time.Time) error
	RevokeByAgent(agentID string, at time.Time) error
}

type AgentStore interface {
	Create(agent *models.Agent) error
	ListByUser(userID string) ([]models.Agent, error)
	GetByAgentIDForUser(userID, agentID string) (*models.Agent, error)
	UpdateStatusForUser(userID, agentID, name, status string, at time.Time) (*models.Agent, error)
}
