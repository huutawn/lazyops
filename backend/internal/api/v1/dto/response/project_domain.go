package response

import "time"

type ProjectDomainResponse struct {
	ID                 string    `json:"id"`
	ProjectID          string    `json:"project_id"`
	Hostname           string    `json:"hostname"`
	Label              string    `json:"label"`
	Kind               string    `json:"kind"`
	Status             string    `json:"status"`
	StatusReason       string    `json:"status_reason,omitempty"`
	CloudflareRecordID string    `json:"cloudflare_record_id,omitempty"`
	TargetKind         string    `json:"target_kind,omitempty"`
	TargetID           string    `json:"target_id,omitempty"`
	LastSyncedIP       string    `json:"last_synced_ip,omitempty"`
	PublicURL          string    `json:"public_url,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
