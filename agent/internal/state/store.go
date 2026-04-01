package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load(_ context.Context) (*AgentLocalState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			state := DefaultState()
			return &state, nil
		}
		return nil, err
	}

	var current AgentLocalState
	if err := json.Unmarshal(payload, &current); err != nil {
		return nil, err
	}
	return &current, nil
}

func (s *Store) Save(_ context.Context, current *AgentLocalState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func (s *Store) Update(ctx context.Context, fn func(*AgentLocalState) error) (*AgentLocalState, error) {
	current, err := s.Load(ctx)
	if err != nil {
		return nil, err
	}
	if err := fn(current); err != nil {
		return nil, err
	}
	if err := s.Save(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

type AgentLocalState struct {
	Metadata           AgentMetadata           `json:"metadata"`
	Enrollment         EnrollmentState         `json:"enrollment"`
	RevisionCache      RevisionCache           `json:"revision_cache"`
	Health             HealthSnapshotState     `json:"health"`
	CapabilitySnapshot CapabilitySnapshotState `json:"capability_snapshot"`
}

type AgentMetadata struct {
	AgentID       string                `json:"agent_id,omitempty"`
	Hostname      string                `json:"hostname,omitempty"`
	AgentKind     contracts.AgentKind   `json:"agent_kind,omitempty"`
	RuntimeMode   contracts.RuntimeMode `json:"runtime_mode,omitempty"`
	CurrentState  contracts.AgentState  `json:"current_state"`
	LastStartedAt time.Time             `json:"last_started_at,omitempty"`
	LastStoppedAt time.Time             `json:"last_stopped_at,omitempty"`
}

type EnrollmentState struct {
	Enrolled                  bool      `json:"enrolled"`
	SessionID                 string    `json:"session_id,omitempty"`
	LastEnrollmentAt          time.Time `json:"last_enrollment_at,omitempty"`
	LastBootstrapAt           time.Time `json:"last_bootstrap_at,omitempty"`
	TokenReference            string    `json:"token_reference,omitempty"`
	EncryptedAgentToken       string    `json:"encrypted_agent_token,omitempty"`
	BootstrapTokenFingerprint string    `json:"bootstrap_token_fingerprint,omitempty"`
	BootstrapTokenUsed        bool      `json:"bootstrap_token_used"`
}

type RevisionCache struct {
	CurrentRevisionID      string    `json:"current_revision_id,omitempty"`
	StableRevisionID       string    `json:"stable_revision_id,omitempty"`
	PendingRevisionID      string    `json:"pending_revision_id,omitempty"`
	CandidateRevisionID    string    `json:"candidate_revision_id,omitempty"`
	CandidateState         string    `json:"candidate_state,omitempty"`
	CandidateWorkspaceRoot string    `json:"candidate_workspace_root,omitempty"`
	LastHealthGateAt       time.Time `json:"last_health_gate_at,omitempty"`
	LastHealthGateState    string    `json:"last_health_gate_state,omitempty"`
	LastHealthGateSummary  string    `json:"last_health_gate_summary,omitempty"`
	LastPolicyAction       string    `json:"last_policy_action,omitempty"`
	UpdatedAt              time.Time `json:"updated_at,omitempty"`
}

type HealthSnapshotState struct {
	Status    contracts.AgentHealthStatus `json:"status,omitempty"`
	Summary   string                      `json:"summary,omitempty"`
	UpdatedAt time.Time                   `json:"updated_at,omitempty"`
}

type CapabilitySnapshotState struct {
	LastComputedAt          time.Time                         `json:"last_computed_at,omitempty"`
	Fingerprint             string                            `json:"fingerprint,omitempty"`
	Version                 int64                             `json:"version,omitempty"`
	LastReportedAt          time.Time                         `json:"last_reported_at,omitempty"`
	LastReportedFingerprint string                            `json:"last_reported_fingerprint,omitempty"`
	LastReportedVersion     int64                             `json:"last_reported_version,omitempty"`
	Payload                 contracts.CapabilityReportPayload `json:"payload"`
	LastReportedPayload     contracts.CapabilityReportPayload `json:"last_reported_payload"`
}

func DefaultState() AgentLocalState {
	return AgentLocalState{
		Metadata: AgentMetadata{
			CurrentState: contracts.AgentStateBootstrap,
		},
		Health: HealthSnapshotState{
			Status: contracts.AgentHealthOffline,
		},
	}
}
