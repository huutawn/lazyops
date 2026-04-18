package request

type CreateDeploymentRequest struct {
	BlueprintID string `json:"blueprint_id,omitempty"`
	TriggerKind string `json:"trigger_kind"`
	ServiceIDs  []string `json:"service_ids,omitempty"`
}

type DeploymentActionRequest struct {
	Action string `json:"action"`
}
