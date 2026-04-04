package models

import "time"

type PublicRoute struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string    `json:"project_id" gorm:"size:64;not null;index"`
	DeploymentID string    `json:"deployment_id" gorm:"size:64;not null;index"`
	ServiceName  string    `json:"service_name" gorm:"size:255;not null"`
	Domain       string    `json:"domain" gorm:"size:512;not null;index"`
	DomainKind   string    `json:"domain_kind" gorm:"size:64;not null"`
	PathPrefix   string    `json:"path_prefix" gorm:"size:512;not null;default:'/'"`
	UpstreamPort int       `json:"upstream_port" gorm:"not null"`
	HTTPS        bool      `json:"https" gorm:"not null;default:true"`
	Status       string    `json:"status" gorm:"size:64;not null;default:'active';index"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type GatewayConfigIntent struct {
	ID             string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID      string     `json:"project_id" gorm:"size:64;not null;index"`
	DeploymentID   string     `json:"deployment_id" gorm:"size:64;not null;index"`
	RevisionID     string     `json:"revision_id" gorm:"size:64;not null;index"`
	TargetKind     string     `json:"target_kind" gorm:"size:64;not null"`
	TargetID       string     `json:"target_id" gorm:"size:64;not null"`
	ConfigJSON     string     `json:"config_json" gorm:"type:jsonb;not null"`
	Status         string     `json:"status" gorm:"size:64;not null;default:'pending';index"`
	DispatchedAt   *time.Time `json:"dispatched_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	CreatedAt      time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ReleaseHistory struct {
	ID           string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string     `json:"project_id" gorm:"size:64;not null;index"`
	DeploymentID string     `json:"deployment_id" gorm:"size:64;not null;index"`
	RevisionID   string     `json:"revision_id" gorm:"size:64;not null;index"`
	CommitSHA    string     `json:"commit_sha" gorm:"size:255;not null;index"`
	TriggerKind  string     `json:"trigger_kind" gorm:"size:128;not null"`
	Status       string     `json:"status" gorm:"size:64;not null;index"`
	RuntimeMode  string     `json:"runtime_mode" gorm:"size:64;not null"`
	SummaryJSON  string     `json:"summary_json" gorm:"type:jsonb;not null"`
	DeployedAt   *time.Time `json:"deployed_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"index"`
}
