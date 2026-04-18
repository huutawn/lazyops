package request

type CreateMeshNetworkRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	CIDR     string `json:"cidr"`
}

type CreateClusterRequest struct {
	Name                string `json:"name"`
	Provider            string `json:"provider"`
	KubeconfigSecretRef string `json:"kubeconfig_secret_ref"`
}

type ConnectClusterNodeSSHRequest struct {
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
