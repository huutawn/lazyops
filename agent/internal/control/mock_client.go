package control

import (
	"context"
	"log/slog"
	"sync"

	"lazyops-agent/internal/contracts"
)

type MockClient struct {
	logger      *slog.Logger
	mu          sync.Mutex
	connected   bool
	sessionAuth contracts.SessionAuthPayload
	transcript  []contracts.CommandEnvelope
	bootstrap   *bootstrapRegistry
}

func NewMockClient(logger *slog.Logger) *MockClient {
	return &MockClient{
		logger:    logger,
		bootstrap: newDefaultBootstrapRegistry(),
	}
}

func (c *MockClient) Enroll(ctx context.Context, req contracts.EnrollAgentRequest) (contracts.EnrollAgentResponse, error) {
	response, err := c.bootstrap.Enroll(ctx, req)
	if err != nil {
		return contracts.EnrollAgentResponse{}, err
	}

	c.logger.Info("mock enroll agent",
		"bootstrap_token", req.BootstrapToken,
		"target_ref", req.Machine.Labels["target_ref"],
		"runtime_mode", req.RuntimeMode,
		"agent_kind", req.AgentKind,
	)

	return response, nil
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
	return c.recordEnvelope(handshakeEnvelopeType, payload.Auth.AgentID, payload)
}

func (c *MockClient) SendHeartbeat(_ context.Context, payload contracts.HeartbeatPayload) error {
	return c.recordEnvelope(heartbeatEnvelopeType, payload.AgentID, payload)
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

func (c *MockClient) recordEnvelope(messageType contracts.CommandType, agentID string, payload any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return ErrControlClientNotConnected
	}

	envelope, _, err := buildEnvelope(messageType, agentID, payload)
	if err != nil {
		return err
	}

	c.transcript = append(c.transcript, envelope)
	return nil
}
