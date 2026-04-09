package response

import "time"

type BootstrapAutoAcceptedResponse struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	ProjectID string `json:"project_id"`
}

type BootstrapStepActionResponse struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Href     string `json:"href,omitempty"`
	Method   string `json:"method,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

type BootstrapStepResponse struct {
	ID      string                        `json:"id"`
	State   string                        `json:"state"`
	Summary string                        `json:"summary"`
	Actions []BootstrapStepActionResponse `json:"actions"`
}

type BootstrapAutoModeResponse struct {
	Enabled              bool   `json:"enabled"`
	SelectedMode         string `json:"selected_mode"`
	ModeSource           string `json:"mode_source"`
	ModeReasonCode       string `json:"mode_reason_code"`
	ModeReasonHuman      string `json:"mode_reason_human"`
	UpshiftAllowed       bool   `json:"upshift_allowed"`
	DownshiftAllowed     bool   `json:"downshift_allowed"`
	DownshiftBlockReason string `json:"downshift_block_reason"`
}

type BootstrapInventoryResponse struct {
	HealthyInstances    int `json:"healthy_instances"`
	HealthyMeshNetworks int `json:"healthy_mesh_networks"`
	HealthyK3sClusters  int `json:"healthy_k3s_clusters"`
}

type BootstrapStatusResponse struct {
	ProjectID    string                     `json:"project_id"`
	OverallState string                     `json:"overall_state"`
	Steps        []BootstrapStepResponse    `json:"steps"`
	AutoMode     BootstrapAutoModeResponse  `json:"auto_mode"`
	Inventory    BootstrapInventoryResponse `json:"inventory"`
	UpdatedAt    time.Time                  `json:"updated_at"`
}

type BootstrapPipelineEventResponse struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Label     string    `json:"label"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type BootstrapOneClickDeployResponse struct {
	ProjectID     string                           `json:"project_id"`
	BlueprintID   string                           `json:"blueprint_id"`
	RevisionID    string                           `json:"revision_id"`
	DeploymentID  string                           `json:"deployment_id"`
	RolloutStatus string                           `json:"rollout_status"`
	RolloutReason string                           `json:"rollout_reason,omitempty"`
	CorrelationID string                           `json:"correlation_id,omitempty"`
	AgentID       string                           `json:"agent_id,omitempty"`
	Timeline      []BootstrapPipelineEventResponse `json:"timeline"`
}
