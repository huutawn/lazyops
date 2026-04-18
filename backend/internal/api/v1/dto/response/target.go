package response

import "time"

type MeshNetworkSummaryResponse struct {
	ID         string    `json:"id"`
	TargetKind string    `json:"target_kind"`
	Name       string    `json:"name"`
	Provider   string    `json:"provider"`
	CIDR       string    `json:"cidr"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MeshNetworkListResponse struct {
	Items []MeshNetworkSummaryResponse `json:"items"`
}

type ClusterSummaryResponse struct {
	ID         string    `json:"id"`
	TargetKind string    `json:"target_kind"`
	Name       string    `json:"name"`
	Provider   string    `json:"provider"`
	Status     string    `json:"status"`
	PublicIP   *string   `json:"public_ip,omitempty"`
	InstanceID *string   `json:"instance_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ClusterListResponse struct {
	Items []ClusterSummaryResponse `json:"items"`
}

type ClusterNodeResponse struct {
	ClusterID   string            `json:"cluster_id"`
	InstanceID  string            `json:"instance_id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	K8sNodeName string            `json:"k8s_node_name,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	LastSeenAt  *time.Time        `json:"last_seen_at,omitempty"`
	IsReady     bool              `json:"is_ready"`
}

type ClusterNodeListResponse struct {
	Items []ClusterNodeResponse `json:"items"`
}

type ConnectClusterNodeSSHResponse struct {
	ClusterID string                  `json:"cluster_id"`
	Instance  InstanceSummaryResponse `json:"instance"`
	Join      struct {
		ClusterID           string                    `json:"cluster_id"`
		InstanceID          string                    `json:"instance_id"`
		StartedAt           time.Time                 `json:"started_at"`
		HostKeyFingerprint  string                    `json:"host_key_fingerprint,omitempty"`
		NodeName            string                    `json:"node_name,omitempty"`
		JoinServerURL       string                    `json:"join_server_url,omitempty"`
		LabeledByControl    bool                      `json:"labeled_by_control"`
		PlacementLabelKey   string                    `json:"placement_label_key,omitempty"`
		PlacementLabelValue string                    `json:"placement_label_value,omitempty"`
		Stages              []BootstrapStepResponse   `json:"stages,omitempty"`
	} `json:"join"`
}
