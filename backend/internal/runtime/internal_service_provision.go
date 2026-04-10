package runtime

type InternalServiceProvisionSpec struct {
	Kind          string `json:"kind"`
	Alias         string `json:"alias"`
	Protocol      string `json:"protocol"`
	Port          int    `json:"port"`
	LocalEndpoint string `json:"local_endpoint"`
}

type ProvisionInternalServicesPayload struct {
	ProjectID string                         `json:"project_id"`
	BindingID string                         `json:"binding_id,omitempty"`
	Services  []InternalServiceProvisionSpec `json:"services"`
}
