package service

import "time"

const (
	ProjectDomainKindManaged = "managed"

	ProjectDomainStatusPending = "pending"
	ProjectDomainStatusActive  = "active"
	ProjectDomainStatusError   = "error"
)

type ProjectDomainRecord struct {
	ID                 string
	ProjectID          string
	Hostname           string
	Label              string
	Kind               string
	Status             string
	StatusReason       string
	CloudflareRecordID string
	TargetKind         string
	TargetID           string
	LastSyncedIP       string
	PublicURL          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AllocateProjectDomainCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	Regenerate      bool
}

type RenameProjectDomainCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	Label           string
}
