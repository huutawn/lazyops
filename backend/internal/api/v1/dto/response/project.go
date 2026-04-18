package response

import "time"

type ProjectSummaryResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	NamespaceSlug string    `json:"namespace_slug"`
	ClusterID     string    `json:"cluster_id,omitempty"`
	RuntimeMode   string    `json:"runtime_mode"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProjectListResponse struct {
	Items []ProjectSummaryResponse `json:"items"`
}

type ProjectServiceListResponse struct {
	Items []ProjectServiceResponse `json:"items"`
}

type PlacementNodeListResponse struct {
	ClusterID string                `json:"cluster_id,omitempty"`
	Items     []ClusterNodeResponse `json:"items"`
}

type ProjectRuntimeLogPreviewResponse struct {
	ID            string    `json:"id,omitempty"`
	Source        string    `json:"source,omitempty"`
	Level         string    `json:"level"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
	Node          string    `json:"node,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
}

type ProjectRuntimeDependencyResponse struct {
	ServiceID        string `json:"service_id,omitempty"`
	ServiceName      string `json:"service_name"`
	Status           string `json:"status"`
	StatusReason     string `json:"status_reason,omitempty"`
	InternalEndpoint string `json:"internal_endpoint,omitempty"`
}

type ProjectRuntimeNodeResponse struct {
	ClusterID   string            `json:"cluster_id"`
	InstanceID  string            `json:"instance_id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	K8sNodeName string            `json:"k8s_node_name,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	LastSeenAt  *time.Time        `json:"last_seen_at,omitempty"`
	IsReady     bool              `json:"is_ready"`
}

type ProjectRuntimeServiceResponse struct {
	ServiceID         string                             `json:"service_id"`
	Name              string                             `json:"name"`
	Kind              string                             `json:"kind,omitempty"`
	SourceType        string                             `json:"source_type,omitempty"`
	Public            bool                               `json:"public"`
	RuntimeProfile    string                             `json:"runtime_profile,omitempty"`
	RuntimeStatus     string                             `json:"runtime_status"`
	RuntimeReason     string                             `json:"runtime_reason,omitempty"`
	BuildState        string                             `json:"build_state,omitempty"`
	RolloutState      string                             `json:"rollout_state,omitempty"`
	PlacementMode     string                             `json:"placement_mode,omitempty"`
	RequestedNodeID   string                             `json:"requested_node_id,omitempty"`
	EffectiveNodeIDs  []string                           `json:"effective_node_ids,omitempty"`
	ImageRef          string                             `json:"image_ref,omitempty"`
	ImageDigest       string                             `json:"image_digest,omitempty"`
	RevisionID        string                             `json:"revision_id,omitempty"`
	Revision          int                                `json:"revision,omitempty"`
	DeploymentID      string                             `json:"deployment_id,omitempty"`
	PublicURLs        []string                           `json:"public_urls,omitempty"`
	InternalEndpoints []string                           `json:"internal_endpoints,omitempty"`
	Dependencies      []ProjectRuntimeDependencyResponse `json:"dependencies,omitempty"`
	RecentLogs        []ProjectRuntimeLogPreviewResponse `json:"recent_logs,omitempty"`
}

type ProjectRuntimeSummaryResponse struct {
	ProjectID        string                          `json:"project_id"`
	RuntimeMode      string                          `json:"runtime_mode,omitempty"`
	ClusterID        string                          `json:"cluster_id,omitempty"`
	Namespace        string                          `json:"namespace,omitempty"`
	LiveRevisionID   string                          `json:"live_revision_id,omitempty"`
	LiveRevision     int                             `json:"live_revision,omitempty"`
	StableRevisionID string                          `json:"stable_revision_id,omitempty"`
	StableRevision   int                             `json:"stable_revision,omitempty"`
	SyncState        string                          `json:"sync_state"`
	SyncReason       string                          `json:"sync_reason,omitempty"`
	PublicURLs       []string                        `json:"public_urls,omitempty"`
	Nodes            []ProjectRuntimeNodeResponse    `json:"nodes,omitempty"`
	Services         []ProjectRuntimeServiceResponse `json:"services,omitempty"`
}

type ProjectServiceActionResponse struct {
	Action       string `json:"action"`
	ServiceID    string `json:"service_id"`
	ServiceName  string `json:"service_name"`
	Status       string `json:"status"`
	TriggerKind  string `json:"trigger_kind,omitempty"`
	DeploymentID string `json:"deployment_id,omitempty"`
	RevisionID   string `json:"revision_id,omitempty"`
	Message      string `json:"message,omitempty"`
}

type ProjectRepoLinkResponse struct {
	ID                   string    `json:"id"`
	ProjectID            string    `json:"project_id"`
	GitHubInstallationID int64     `json:"github_installation_id"`
	GitHubRepoID         int64     `json:"github_repo_id"`
	RepoOwner            string    `json:"repo_owner"`
	RepoName             string    `json:"repo_name"`
	RepoFullName         string    `json:"repo_full_name"`
	TrackedBranch        string    `json:"tracked_branch"`
	PreviewEnabled       bool      `json:"preview_enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
