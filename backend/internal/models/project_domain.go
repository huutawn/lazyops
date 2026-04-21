package models

import "time"

type ProjectDomain struct {
	ID                 string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID          string    `json:"project_id" gorm:"size:64;not null;index;uniqueIndex:idx_project_domains_project_kind"`
	Hostname           string    `json:"hostname" gorm:"size:512;not null;uniqueIndex"`
	Label              string    `json:"label" gorm:"size:63;not null"`
	Kind               string    `json:"kind" gorm:"size:32;not null;default:'managed';uniqueIndex:idx_project_domains_project_kind"`
	Status             string    `json:"status" gorm:"size:32;not null;default:'pending';index"`
	StatusReason       string    `json:"status_reason,omitempty" gorm:"size:1024;not null;default:''"`
	CloudflareRecordID string    `json:"cloudflare_record_id,omitempty" gorm:"size:255;not null;default:''"`
	TargetKind         string    `json:"target_kind,omitempty" gorm:"size:64;not null;default:''"`
	TargetID           string    `json:"target_id,omitempty" gorm:"size:64;not null;default:''"`
	LastSyncedIP       string    `json:"last_synced_ip,omitempty" gorm:"size:64;not null;default:''"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
