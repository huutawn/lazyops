package service

import "time"

type RegisterCommand struct {
	Name     string
	Email    string
	Password string
	Role     string
}

type LoginCommand struct {
	Email    string
	Password string
}

type UserProfile struct {
	ID    uint
	Name  string
	Email string
	Role  string
}

type AuthResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   time.Duration
	User        UserProfile
}

type CreateAgentCommand struct {
	AgentID string
	Name    string
	Status  string
}

type UpdateAgentStatusCommand struct {
	AgentID string
	Name    string
	Status  string
	Source  string
	At      time.Time
}

type AgentRecord struct {
	ID         uint
	AgentID    string
	Name       string
	Status     string
	LastSeenAt *time.Time
	UpdatedAt  time.Time
}

type RealtimeMeta struct {
	Source string
	At     time.Time
}

type AgentRealtimeEvent struct {
	Type    string
	Payload AgentRecord
	Meta    RealtimeMeta
}
