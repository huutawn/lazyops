package request

type CreateInstanceRequest struct {
	Name      string            `json:"name"`
	PublicIP  string            `json:"public_ip"`
	PrivateIP string            `json:"private_ip"`
	Labels    map[string]string `json:"labels"`
}

type InstallInstanceAgentSSHRequest struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	PrivateKey         string `json:"private_key"`
	HostKeyFingerprint string `json:"host_key_fingerprint"`
	ControlPlaneURL    string `json:"control_plane_url"`
	RuntimeMode        string `json:"runtime_mode"`
	AgentKind          string `json:"agent_kind"`
	AgentImage         string `json:"agent_image"`
	ContainerName      string `json:"container_name"`
}
