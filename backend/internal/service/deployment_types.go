package service

import "time"

type CreateDeploymentCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	BlueprintID     string
	TriggerKind     string
	SourceRef       string
	Artifact        BlueprintArtifactMetadata
	ServiceIDs      []string
}

type DesiredStateRevisionRecord struct {
	ID                   string
	ProjectID            string
	ProjectSlug          string
	BlueprintID          string
	DeploymentBindingID  string
	Namespace            string
	CommitSHA            string
	ArtifactRef          string
	ImageRef             string
	TriggerKind          string
	Status               string
	RuntimeMode          string
	Services             []BlueprintServiceContractRecord
	ServiceSpecs         []K3sServiceSpecRecord
	DependencyBindings   []LazyopsYAMLDependencyBinding
	InternalBindings     []InternalBindingRecord
	CompatibilityPolicy  LazyopsYAMLCompatibilityPolicy
	MagicDomainPolicy    LazyopsYAMLMagicDomainPolicy
	ScaleToZeroPolicy    LazyopsYAMLScaleToZeroPolicy
	ManifestBundle       K3sManifestBundleRecord
	PublicDomains        []PublicDomainRecord
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
	Namespace            string
	RuntimeMode          string
	Services             []BlueprintServiceContractRecord
	ServiceSpecs         []K3sServiceSpecRecord
	PlacementAssignments []PlacementAssignmentRecord
	PublicURLs           []string
	PublicURLReason      string
	StartedAt            *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type DeploymentDetailRecord struct {
	DeploymentOverviewRecord
	Timeline        []DeploymentTimelineEventRecord
	CanRollback     bool
	CanPromote      bool
	CanCancel       bool
	SafetyPolicy    DeploymentSafetyPolicyRecord
	IncidentSummary *DeploymentIncidentSummaryRecord
}

type DeploymentSafetyPolicyRecord struct {
	AutoRollbackEnabled bool     `json:"auto_rollback_enabled"`
	Triggers            []string `json:"triggers"`
	Description         string   `json:"description"`
}

type DeploymentFixActionRecord struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Href   string `json:"href"`
	Method string `json:"method"`
}

type DeploymentIncidentSummaryRecord struct {
	State         string                     `json:"state"`
	Headline      string                     `json:"headline"`
	Reason        string                     `json:"reason"`
	Recommended   string                     `json:"recommended"`
	IncidentID    string                     `json:"incident_id,omitempty"`
	IncidentKind  string                     `json:"incident_kind,omitempty"`
	IncidentLevel string                     `json:"incident_level,omitempty"`
	PrimaryAction *DeploymentFixActionRecord `json:"primary_action,omitempty"`
}
