package service

import "time"

type CreateDeploymentCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	BlueprintID     string
	TriggerKind     string
}

type DesiredStateRevisionRecord struct {
	ID                   string
	ProjectID            string
	BlueprintID          string
	DeploymentBindingID  string
	CommitSHA            string
	ArtifactRef          string
	ImageRef             string
	TriggerKind          string
	Status               string
	RuntimeMode          string
	Services             []BlueprintServiceContractRecord
	DependencyBindings   []LazyopsYAMLDependencyBinding
	CompatibilityPolicy  LazyopsYAMLCompatibilityPolicy
	MagicDomainPolicy    LazyopsYAMLMagicDomainPolicy
	ScaleToZeroPolicy    LazyopsYAMLScaleToZeroPolicy
	PlacementAssignments []PlacementAssignmentRecord
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type DeploymentRecord struct {
	ID          string
	ProjectID   string
	RevisionID  string
	Status      string
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateDeploymentResult struct {
	Revision   DesiredStateRevisionRecord
	Deployment DeploymentRecord
}
