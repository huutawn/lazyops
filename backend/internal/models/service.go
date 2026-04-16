package models

import "time"

type Service struct {
	ID                 string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID          string    `json:"project_id" gorm:"size:64;not null;index;uniqueIndex:idx_services_project_name"`
	Name               string    `json:"name" gorm:"size:255;not null;uniqueIndex:idx_services_project_name"`
	Path               string    `json:"path" gorm:"size:1024;not null"`
	Kind               string    `json:"kind" gorm:"size:64;not null;default:'app';index"`
	Public             bool      `json:"public" gorm:"not null;default:false"`
	RuntimeProfile     *string   `json:"runtime_profile,omitempty" gorm:"size:128"`
	StartHint          string    `json:"start_hint" gorm:"size:1024;not null;default:''"`
	ImageRef           string    `json:"image_ref" gorm:"size:1024;not null;default:''"`
	ImageDigest        string    `json:"image_digest" gorm:"size:255;not null;default:''"`
	DetectedPortsJSON  string    `json:"detected_ports_json" gorm:"type:jsonb;not null;default:'[]'"`
	TargetPort         int       `json:"target_port" gorm:"not null;default:0"`
	ServicePort        int       `json:"service_port" gorm:"not null;default:0"`
	Replicas           int       `json:"replicas" gorm:"not null;default:1"`
	EnvBundleJSON      string    `json:"env_bundle_json" gorm:"type:jsonb;not null;default:'{}'"`
	PVCSpecJSON        string    `json:"pvc_spec_json" gorm:"type:jsonb;not null;default:'{}'"`
	DeployStrategyJSON string    `json:"deploy_strategy_json" gorm:"type:jsonb;not null;default:'{}'"`
	HealthcheckJSON    string    `json:"healthcheck_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
