package response

import "time"

type DeploymentBindingResponse struct {
	ID                  string         `json:"id"`
	ProjectID           string         `json:"project_id"`
	Name                string         `json:"name"`
	TargetRef           string         `json:"target_ref"`
	RuntimeMode         string         `json:"runtime_mode"`
	TargetKind          string         `json:"target_kind"`
	TargetID            string         `json:"target_id"`
	PlacementPolicy     map[string]any `json:"placement_policy"`
	DomainPolicy        map[string]any `json:"domain_policy"`
	CompatibilityPolicy map[string]any `json:"compatibility_policy"`
	ScaleToZeroPolicy   map[string]any `json:"scale_to_zero_policy"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type DeploymentBindingListResponse struct {
	Items []DeploymentBindingResponse `json:"items"`
}
