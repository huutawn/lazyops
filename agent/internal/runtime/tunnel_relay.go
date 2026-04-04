package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type TunnelRelayConfig struct {
	MaxActiveTunnels int
	SessionTimeout   time.Duration
}

func DefaultTunnelRelayConfig() TunnelRelayConfig {
	return TunnelRelayConfig{
		MaxActiveTunnels: 5,
		SessionTimeout:   30 * time.Minute,
	}
}

type TunnelSession struct {
	TunnelID    string    `json:"tunnel_id"`
	ProjectID   string    `json:"project_id"`
	BindingID   string    `json:"binding_id"`
	RevisionID  string    `json:"revision_id"`
	ServiceName string    `json:"service_name"`
	TargetPort  int       `json:"target_port"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Status      string    `json:"status"`
}

type TunnelRelay struct {
	logger *slog.Logger
	cfg    TunnelRelayConfig
	now    func() time.Time

	mu       sync.Mutex
	sessions map[string]*TunnelSession
	total    int
	expired  int
}

func NewTunnelRelay(logger *slog.Logger, cfg TunnelRelayConfig) *TunnelRelay {
	if cfg.MaxActiveTunnels <= 0 {
		cfg.MaxActiveTunnels = 5
	}
	if cfg.SessionTimeout <= 0 {
		cfg.SessionTimeout = 30 * time.Minute
	}

	return &TunnelRelay{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		sessions: make(map[string]*TunnelSession),
	}
}

func (r *TunnelRelay) CreateSession(tunnelID, projectID, bindingID, revisionID, serviceName string, targetPort int) (*TunnelSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.total++

	if len(r.sessions) >= r.cfg.MaxActiveTunnels {
		return nil, fmt.Errorf("max active tunnels reached (%d)", r.cfg.MaxActiveTunnels)
	}

	now := r.now()
	session := &TunnelSession{
		TunnelID:    tunnelID,
		ProjectID:   projectID,
		BindingID:   bindingID,
		RevisionID:  revisionID,
		ServiceName: serviceName,
		TargetPort:  targetPort,
		CreatedAt:   now,
		ExpiresAt:   now.Add(r.cfg.SessionTimeout),
		Status:      "active",
	}

	r.sessions[tunnelID] = session
	return session, nil
}

func (r *TunnelRelay) CloseSession(tunnelID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[tunnelID]
	if !exists {
		return false
	}

	session.Status = "closed"
	delete(r.sessions, tunnelID)
	return true
}

func (r *TunnelRelay) CollectExpiredSessions() []*TunnelSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	var expired []*TunnelSession

	for id, session := range r.sessions {
		if now.After(session.ExpiresAt) {
			session.Status = "expired"
			expired = append(expired, session)
			delete(r.sessions, id)
			r.expired++
		}
	}

	return expired
}

func (r *TunnelRelay) ActiveSessions() []*TunnelSession {
	r.mu.Lock()
	defer r.mu.Unlock()

	sessions := make([]*TunnelSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

func (r *TunnelRelay) Stats() (total, active, expired int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.total, len(r.sessions), r.expired
}

func (r *TunnelRelay) PersistTunnelState(workspaceRoot, projectID, bindingID string) (string, error) {
	tunnelDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "tunnels")
	if err := os.MkdirAll(tunnelDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create tunnel directory: %w", err)
	}

	sessions := r.ActiveSessions()
	timestamp := r.now().Format("20060102T150405Z")
	statePath := filepath.Join(tunnelDir, "state_"+timestamp+".json")

	raw, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal tunnel sessions: %w", err)
	}

	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write tunnel state: %w", err)
	}

	return statePath, nil
}

type ReportTunnelStatePayload struct {
	ProjectID     string                `json:"project_id"`
	BindingID     string                `json:"binding_id"`
	RevisionID    string                `json:"revision_id"`
	RuntimeMode   contracts.RuntimeMode `json:"runtime_mode"`
	WorkspaceRoot string                `json:"workspace_root"`
}

func (r *TunnelRelay) HandleReportTunnelState(ctx context.Context, logger *slog.Logger, payload ReportTunnelStatePayload) (int, error) {
	if logger == nil {
		logger = slog.Default()
	}

	expired := r.CollectExpiredSessions()
	for _, session := range expired {
		logger.Info("tunnel session expired",
			"tunnel_id", session.TunnelID,
			"service", session.ServiceName,
		)
	}

	active := len(r.ActiveSessions())

	workspaceRoot := payload.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(
			"/var/lib/lazyops",
			"projects", payload.ProjectID,
			"bindings", payload.BindingID,
			"revisions", payload.RevisionID,
		)
	}

	statePath, err := r.PersistTunnelState(workspaceRoot, payload.ProjectID, payload.BindingID)
	if err != nil {
		logger.Warn("could not persist tunnel state",
			"project_id", payload.ProjectID,
			"error", err,
		)
	} else {
		logger.Info("tunnel state persisted",
			"project_id", payload.ProjectID,
			"active", active,
			"expired", len(expired),
			"state_path", statePath,
		)
	}

	return len(expired), nil
}
