package models

import "time"

type AssistantSession struct {
	ID            string     `json:"id" gorm:"primaryKey;size:64"`
	UserID        string     `json:"user_id" gorm:"size:64;not null;index"`
	ProjectID     *string    `json:"project_id,omitempty" gorm:"size:64;index"`
	Title         string     `json:"title" gorm:"size:255;not null;default:''"`
	Status        string     `json:"status" gorm:"size:64;not null;default:'active';index"`
	LastMessageAt time.Time  `json:"last_message_at" gorm:"not null;index"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type AssistantMessage struct {
	ID             string    `json:"id" gorm:"primaryKey;size:64"`
	SessionID      string    `json:"session_id" gorm:"size:64;not null;index"`
	Role           string    `json:"role" gorm:"size:32;not null;index"`
	Kind           string    `json:"kind" gorm:"size:64;not null;index"`
	Content        string    `json:"content" gorm:"type:text;not null"`
	ContentJSON    string    `json:"content_json" gorm:"type:jsonb;not null;default:'{}'"`
	TokenUsageJSON string    `json:"token_usage_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time `json:"created_at" gorm:"index"`
}

type AssistantActionPlan struct {
	ID                   string     `json:"id" gorm:"primaryKey;size:64"`
	SessionID            string     `json:"session_id" gorm:"size:64;not null;index"`
	ProjectID            string     `json:"project_id" gorm:"size:64;not null;index"`
	ActionType           string     `json:"action_type" gorm:"size:64;not null;index"`
	Status               string     `json:"status" gorm:"size:64;not null;index"`
	PlanJSON             string     `json:"plan_json" gorm:"type:jsonb;not null;default:'{}'"`
	PlanHash             string     `json:"plan_hash" gorm:"size:128;not null;default:''"`
	RiskLevel            string     `json:"risk_level" gorm:"size:32;not null;default:'low'"`
	RequiresConfirmation bool       `json:"requires_confirmation" gorm:"not null;default:false"`
	ConfirmedBy          *string    `json:"confirmed_by,omitempty" gorm:"size:64;index"`
	ConfirmedAt          *time.Time `json:"confirmed_at,omitempty"`
	ConfirmationMethod   string     `json:"confirmation_method" gorm:"size:64;not null;default:''"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty" gorm:"index"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type AssistantAuditEvent struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	SessionID    string    `json:"session_id" gorm:"size:64;not null;index"`
	ActionPlanID *string   `json:"action_plan_id,omitempty" gorm:"size:64;index"`
	EventType    string    `json:"event_type" gorm:"size:64;not null;index"`
	PayloadJSON  string    `json:"payload_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
}
