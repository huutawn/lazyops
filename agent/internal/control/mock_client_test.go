package control

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestMockClientRecordsHandshakeAndHeartbeat(t *testing.T) {
	client := NewMockClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	auth := contracts.SessionAuthPayload{
		AgentID:      "agt_local_test",
		AgentToken:   "token-value",
		SessionID:    "sess_123",
		RuntimeMode:  contracts.RuntimeModeStandalone,
		AgentKind:    contracts.AgentKindInstance,
		HandshakeVer: "v0",
		SentAt:       time.Now().UTC(),
	}
	if err := client.Connect(ctx, auth); err != nil {
		t.Fatalf("connect mock client: %v", err)
	}

	if err := client.SendHandshake(ctx, contracts.AgentHandshakePayload{
		Auth: auth,
	}); err != nil {
		t.Fatalf("send handshake: %v", err)
	}
	if err := client.SendHeartbeat(ctx, contracts.HeartbeatPayload{
		AgentID:     auth.AgentID,
		SessionID:   auth.SessionID,
		RuntimeMode: auth.RuntimeMode,
		AgentKind:   auth.AgentKind,
		SentAt:      time.Now().UTC(),
		State:       contracts.AgentStateConnected,
	}); err != nil {
		t.Fatalf("send heartbeat: %v", err)
	}

	transcript := client.Transcript()
	if len(transcript) != 2 {
		t.Fatalf("expected 2 transcript entries, got %d", len(transcript))
	}
	if transcript[0].Type != contracts.CommandType("agent.handshake") {
		t.Fatalf("unexpected first message type %q", transcript[0].Type)
	}
	if transcript[1].Type != contracts.CommandType("agent.heartbeat") {
		t.Fatalf("unexpected second message type %q", transcript[1].Type)
	}
}

func TestMockClientEnrollMarksBootstrapTokenUsed(t *testing.T) {
	client := NewMockClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	req := contracts.EnrollAgentRequest{
		BootstrapToken: "bootstrap-valid-standalone",
		RuntimeMode:    contracts.RuntimeModeStandalone,
		AgentKind:      contracts.AgentKindInstance,
		Machine: contracts.MachineInfo{
			Hostname: "local-dev",
			Labels: map[string]string{
				"target_ref": "local-dev",
			},
		},
	}

	resp, err := client.Enroll(ctx, req)
	if err != nil {
		t.Fatalf("enroll agent: %v", err)
	}
	if resp.AgentID == "" || resp.AgentToken == "" {
		t.Fatal("expected mock enroll response to include agent ID and token")
	}

	_, err = client.Enroll(ctx, req)
	if !errors.Is(err, ErrBootstrapTokenReused) {
		t.Fatalf("expected reused token error, got %v", err)
	}
}
