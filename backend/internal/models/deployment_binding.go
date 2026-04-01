package models

import "time"

type DeploymentBinding struct {
	ID                      string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID               string    `json:"project_id" gorm:"size:64;not null;index;uniqueIndex:idx_deployment_bindings_project_target_ref"`
	Name                    string    `json:"name" gorm:"size:255;not null"`
	TargetRef               string    `json:"target_ref" gorm:"size:255;not null;uniqueIndex:idx_deployment_bindings_project_target_ref"`
	RuntimeMode             string    `json:"runtime_mode" gorm:"size:64;not null"`
	TargetKind              string    `json:"target_kind" gorm:"size:64;not null;index:idx_deployment_bindings_project_target"`
	TargetID                string    `json:"target_id" gorm:"size:64;not null;index:idx_deployment_bindings_project_target"`
	PlacementPolicyJSON     string    `json:"placement_policy_json" gorm:"type:jsonb;not null;default:'{}'"`
	DomainPolicyJSON        string    `json:"domain_policy_json" gorm:"type:jsonb;not null;default:'{}'"`
	CompatibilityPolicyJSON string    `json:"compatibility_policy_json" gorm:"type:jsonb;not null;default:'{}'"`
	ScaleToZeroPolicyJSON   string    `json:"scale_to_zero_policy_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}
