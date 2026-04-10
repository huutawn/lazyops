package response

import "time"

type ProjectInternalServiceResponse struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Kind          string    `json:"kind"`
	Alias         string    `json:"alias"`
	Protocol      string    `json:"protocol"`
	Port          int       `json:"port"`
	LocalEndpoint string    `json:"local_endpoint"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProjectInternalServiceListResponse struct {
	Items []ProjectInternalServiceResponse `json:"items"`
}
