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
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ClusterListResponse struct {
	Items []ClusterSummaryResponse `json:"items"`
}
