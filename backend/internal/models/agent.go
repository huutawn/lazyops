package models

import "time"

type Agent struct {
	ID         string     `json:"id" gorm:"primaryKey;size:64"`
	UserID     string     `json:"user_id" gorm:"size:64;not null;index;uniqueIndex:idx_agents_user_agent_id"`
	AgentID    string     `json:"agent_id" gorm:"size:128;not null;uniqueIndex:idx_agents_user_agent_id"`
	Name       string     `json:"name" gorm:"size:255;not null"`
	Status     string     `json:"status" gorm:"size:64;not null;default:'offline'"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
