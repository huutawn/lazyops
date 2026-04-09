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

type DeploymentTimelineEventRecord struct {
	Timestamp   time.Time
	State       string
	Label       string
	Description string
}

type DeploymentOverviewRecord struct {
	ID                   string
	ProjectID            string
	RevisionID           string
	Revision             int
	CommitSHA            string
	ArtifactRef          string
	ImageRef             string
	TriggerKind          string
	BuildState           string
	RolloutState         string
	Promoted             bool
	TriggeredBy          string
	RuntimeMode          string
	Services             []BlueprintServiceContractRecord
	PlacementAssignments []PlacementAssignmentRecord
	StartedAt            *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type DeploymentDetailRecord struct {
	DeploymentOverviewRecord
	Timeline    []DeploymentTimelineEventRecord
	CanRollback bool
	CanPromote  bool
	CanCancel   bool
}
