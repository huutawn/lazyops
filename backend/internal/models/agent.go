package models

import "time"

type Agent struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	AgentID    string     `json:"agent_id" gorm:"size:128;uniqueIndex;not null"`
	Name       string     `json:"name" gorm:"size:255;not null"`
	Status     string     `json:"status" gorm:"size:64;not null;default:'offline'"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
