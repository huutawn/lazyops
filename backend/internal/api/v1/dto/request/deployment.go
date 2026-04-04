package request

type CreateDeploymentRequest struct {
	BlueprintID string `json:"blueprint_id"`
	TriggerKind string `json:"trigger_kind"`
}
