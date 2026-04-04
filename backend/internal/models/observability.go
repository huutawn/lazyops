package models

import "time"

type TraceSummary struct {
	ID             string    `json:"id" gorm:"primaryKey;size:64"`
	CorrelationID  string    `json:"correlation_id" gorm:"size:255;not null;uniqueIndex"`
	ProjectID      string    `json:"project_id" gorm:"size:64;not null;index"`
	ServiceName    string    `json:"service_name" gorm:"size:255;not null;index"`
	Operation      string    `json:"operation" gorm:"size:512;not null"`
	HTTPMethod     string    `json:"http_method" gorm:"size:16"`
	HTTPStatusCode int       `json:"http_status_code" gorm:"not null"`
	DurationMs     float64   `json:"duration_ms" gorm:"not null"`
	Status         string    `json:"status" gorm:"size:64;not null;index"`
	ErrorSummary   string    `json:"error_summary" gorm:"size:1024"`
	SpanCount      int       `json:"span_count" gorm:"not null"`
	MetadataJSON   string    `json:"metadata_json" gorm:"type:jsonb;not null;default:'{}'"`
	ReceivedAt     time.Time `json:"received_at" gorm:"not null;index"`
	CreatedAt      time.Time `json:"created_at" gorm:"index"`
}

type TopologyNode struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string    `json:"project_id" gorm:"size:64;not null;index"`
	NodeKind     string    `json:"node_kind" gorm:"size:64;not null;index"`
	NodeRef      string    `json:"node_ref" gorm:"size:255;not null;index"`
	Name         string    `json:"name" gorm:"size:255;not null"`
	Status       string    `json:"status" gorm:"size:64;not null;index"`
	MetadataJSON string    `json:"metadata_json" gorm:"type:jsonb;not null;default:'{}'"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type TopologyEdge struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string    `json:"project_id" gorm:"size:64;not null;index"`
	SourceID     string    `json:"source_id" gorm:"size:64;not null;index"`
	TargetID     string    `json:"target_id" gorm:"size:64;not null;index"`
	EdgeKind     string    `json:"edge_kind" gorm:"size:64;not null"`
	Protocol     string    `json:"protocol" gorm:"size:64;not null"`
	MetadataJSON string    `json:"metadata_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt    time.Time `json:"created_at"`
}

type MetricRollup struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string    `json:"project_id" gorm:"size:64;not null;index"`
	ServiceName  string    `json:"service_name" gorm:"size:255;not null;index"`
	MetricKind   string    `json:"metric_kind" gorm:"size:64;not null;index"`
	WindowStart  time.Time `json:"window_start" gorm:"not null;index"`
	WindowEnd    time.Time `json:"window_end" gorm:"not null"`
	P95          float64   `json:"p95" gorm:"not null"`
	Max          float64   `json:"max" gorm:"not null"`
	Min          float64   `json:"min" gorm:"not null"`
	Avg          float64   `json:"avg" gorm:"not null"`
	Count        int64     `json:"count" gorm:"not null"`
	MetadataJSON string    `json:"metadata_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt    time.Time `json:"created_at"`
}
