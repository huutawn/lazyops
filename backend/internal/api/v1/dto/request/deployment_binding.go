package request

type CreateDeploymentBindingRequest struct {
	Name                string         `json:"name"`
	TargetRef           string         `json:"target_ref"`
	RuntimeMode         string         `json:"runtime_mode"`
	TargetKind          string         `json:"target_kind"`
	TargetID            string         `json:"target_id"`
	PlacementPolicy     map[string]any `json:"placement_policy"`
	DomainPolicy        map[string]any `json:"domain_policy"`
	CompatibilityPolicy map[string]any `json:"compatibility_policy"`
	ScaleToZeroPolicy   map[string]any `json:"scale_to_zero_policy"`
}
