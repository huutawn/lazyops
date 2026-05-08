package models

import "time"

type ErrorKnowledgeDocument struct {
	ID            string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID     string    `json:"project_id" gorm:"size:64;not null;index"`
	ServiceName   string    `json:"service_name" gorm:"size:255;not null;index"`
	RevisionID    string    `json:"revision_id" gorm:"size:64;index"`
	CorrelationID string    `json:"correlation_id" gorm:"size:255;index"`
	Fingerprint   string    `json:"fingerprint" gorm:"size:255;not null;index:idx_error_knowledge_fingerprint"`
	Severity      string    `json:"severity" gorm:"size:32;not null;index"`
	Title         string    `json:"title" gorm:"size:512;not null"`
	Body          string    `json:"body" gorm:"type:text;not null"`
	MetadataJSON  string    `json:"metadata_json" gorm:"type:jsonb;not null;default:'{}'"`
	Embedding     string    `json:"embedding" gorm:"type:vector(16);not null"`
	FirstSeenAt   time.Time `json:"first_seen_at" gorm:"not null;index"`
	LastSeenAt    time.Time `json:"last_seen_at" gorm:"not null;index"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
