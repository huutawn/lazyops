package control

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type BootstrapTokenRecord struct {
	AgentID             string
	AgentToken          string
	ExpectedRuntimeMode contracts.RuntimeMode
	ExpectedAgentKind   contracts.AgentKind
	ExpectedTargetRef   string
	ExpiresAt           time.Time
	Used                bool
}

type MockClient struct {
	logger      *slog.Logger
	mu          sync.Mutex
	connected   bool
	sessionAuth contracts.SessionAuthPayload
	transcript  []contracts.CommandEnvelope
	bootstrap   map[string]BootstrapTokenRecord
}

func NewMockClient(logger *slog.Logger) *MockClient {
	return &MockClient{
		logger:    logger,
		bootstrap: defaultBootstrapRegistry(),
	}
}

func (c *MockClient) Enroll(_ context.Context, req contracts.EnrollAgentRequest) (contracts.EnrollAgentResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, ok := c.bootstrap[req.BootstrapToken]
	if !ok {
		return contracts.EnrollAgentResponse{}, ErrBootstrapTokenUnknown
	}
	if time.Now().UTC().After(record.ExpiresAt) {
		return contracts.EnrollAgentResponse{}, ErrBootstrapTokenExpired
	}
	if record.Used {
		return contracts.EnrollAgentResponse{}, ErrBootstrapTokenReused
	}
	if record.ExpectedRuntimeMode != req.RuntimeMode || record.ExpectedAgentKind != req.AgentKind {
		return contracts.EnrollAgentResponse{}, ErrBootstrapTargetMismatch
	}

	targetRef := req.Machine.Labels["target_ref"]
	if record.ExpectedTargetRef != "" && targetRef != record.ExpectedTargetRef {
		return contracts.EnrollAgentResponse{}, ErrBootstrapTargetMismatch
	}

	record.Used = true
	c.bootstrap[req.BootstrapToken] = record
	c.logger.Info("mock enroll agent",
		"bootstrap_token", req.BootstrapToken,
		"target_ref", targetRef,
		"runtime_mode", req.RuntimeMode,
		"agent_kind", req.AgentKind,
	)

	return contracts.EnrollAgentResponse{
		AgentID:    record.AgentID,
		AgentToken: record.AgentToken,
		IssuedAt:   time.Now().UTC(),
	}, nil
}

func (c *MockClient) Connect(_ context.Context, auth contracts.SessionAuthPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = true
	c.sessionAuth = auth
	c.logger.Info("mock control-plane client connected",
		"path", contracts.ControlWebSocketPath,
		"agent_id", auth.AgentID,
		"agent_token", auth.AgentToken,
		"runtime_mode", auth.RuntimeMode,
		"agent_kind", auth.AgentKind,
	)
	return nil
}

func (c *MockClient) SendHandshake(_ context.Context, payload contracts.AgentHandshakePayload) error {
	return c.recordEnvelope(contracts.CommandType("agent.handshake"), payload.Auth.AgentID, "", payload)
}

func (c *MockClient) SendHeartbeat(_ context.Context, payload contracts.HeartbeatPayload) error {
	return c.recordEnvelope(contracts.CommandType("agent.heartbeat"), payload.AgentID, "", payload)
}

func (c *MockClient) Close(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	c.logger.Info("mock control-plane client closed", "agent_id", c.sessionAuth.AgentID)
	return nil
}

func (c *MockClient) Transcript() []contracts.CommandEnvelope {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]contracts.CommandEnvelope, len(c.transcript))
	copy(out, c.transcript)
	return out
}

func (c *MockClient) recordEnvelope(messageType contracts.CommandType, agentID, projectID string, payload any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return errors.New("mock control client is not connected")
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	c.transcript = append(c.transcript, contracts.CommandEnvelope{
		Type:          messageType,
		RequestID:     "",
		CorrelationID: "",
		AgentID:       agentID,
		ProjectID:     projectID,
		Source:        contracts.EnvelopeSourceAgent,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	return nil
}

func defaultBootstrapRegistry() map[string]BootstrapTokenRecord {
	now := time.Now().UTC()
	return map[string]BootstrapTokenRecord{
		"bootstrap-valid-standalone": {
			AgentID:             "agt_enrolled_standalone",
			AgentToken:          "agt-secret-standalone",
			ExpectedRuntimeMode: contracts.RuntimeModeStandalone,
			ExpectedAgentKind:   contracts.AgentKindInstance,
			ExpectedTargetRef:   "local-dev",
			ExpiresAt:           now.Add(1 * time.Hour),
		},
		"bootstrap-valid-k3s": {
			AgentID:             "agt_enrolled_node",
			AgentToken:          "agt-secret-node",
			ExpectedRuntimeMode: contracts.RuntimeModeDistributedK3s,
			ExpectedAgentKind:   contracts.AgentKindNode,
			ExpectedTargetRef:   "k3s-dev",
			ExpiresAt:           now.Add(1 * time.Hour),
		},
		"bootstrap-expired-standalone": {
			AgentID:             "agt_expired",
			AgentToken:          "agt-expired-token",
			ExpectedRuntimeMode: contracts.RuntimeModeStandalone,
			ExpectedAgentKind:   contracts.AgentKindInstance,
			ExpectedTargetRef:   "local-dev",
			ExpiresAt:           now.Add(-1 * time.Hour),
		},
		"bootstrap-reused-standalone": {
			AgentID:             "agt_reused",
			AgentToken:          "agt-reused-token",
			ExpectedRuntimeMode: contracts.RuntimeModeStandalone,
			ExpectedAgentKind:   contracts.AgentKindInstance,
			ExpectedTargetRef:   "local-dev",
			ExpiresAt:           now.Add(1 * time.Hour),
			Used:                true,
		},
	}
}
