package request

type BootstrapAutoRequest struct {
	ProjectID            string `json:"project_id"`
	ProjectName          string `json:"project_name"`
	DefaultBranch        string `json:"default_branch"`
	RepoFullName         string `json:"repo_full_name"`
	GitHubInstallationID int64  `json:"github_installation_id"`
	GitHubRepoID         int64  `json:"github_repo_id"`
	TrackedBranch        string `json:"tracked_branch"`
	InstanceID           string `json:"instance_id"`
	MeshNetworkID        string `json:"mesh_network_id"`
	ClusterID            string `json:"cluster_id"`
	AutoModeEnabled      *bool  `json:"auto_mode_enabled"`
	LockedRuntimeMode    string `json:"locked_runtime_mode"`
}

type BootstrapOneClickDeployRequest struct {
	SourceRef   string `json:"source_ref"`
	CommitSHA   string `json:"commit_sha"`
	ArtifactRef string `json:"artifact_ref"`
	ImageRef    string `json:"image_ref"`
	TriggerKind string `json:"trigger_kind"`
}

type BootstrapConnectInfraSSHRequest struct {
	InstanceName          string            `json:"instance_name"`
	PublicIP              string            `json:"public_ip"`
	PrivateIP             string            `json:"private_ip"`
	Labels                map[string]string `json:"labels"`
	SSHHost               string            `json:"ssh_host"`
	SSHPort               int               `json:"ssh_port"`
	SSHUsername           string            `json:"ssh_username"`
	SSHPassword           string            `json:"ssh_password"`
	SSHPrivateKey         string            `json:"ssh_private_key"`
	SSHHostKeyFingerprint string            `json:"ssh_host_key_fingerprint"`
	ControlPlaneURL       string            `json:"control_plane_url"`
	AgentImage            string            `json:"agent_image"`
	ContainerName         string            `json:"container_name"`
}
