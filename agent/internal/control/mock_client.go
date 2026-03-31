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

type MockClient struct {
	logger      *slog.Logger
	mu          sync.Mutex
	connected   bool
	sessionAuth contracts.SessionAuthPayload
	transcript  []contracts.CommandEnvelope
}

func NewMockClient(logger *slog.Logger) *MockClient {
	return &MockClient{logger: logger}
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
