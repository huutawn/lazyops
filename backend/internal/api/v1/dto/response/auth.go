package response

import "time"

type UserResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int64        `json:"expires_in"`
	User        UserResponse `json:"user"`
}

type CLILoginResponse struct {
	Token     string       `json:"token"`
	TokenType string       `json:"token_type"`
	TokenID   string       `json:"token_id"`
	ExpiresAt *time.Time   `json:"expires_at,omitempty"`
	User      UserResponse `json:"user"`
}

type PATRevokeResponse struct {
	TokenID string `json:"token_id"`
	Revoked bool   `json:"revoked"`
}
