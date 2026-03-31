package dispatcher

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestDispatchSendsAcceptedAndDoneAck(t *testing.T) {
	writer := &fakeWriter{}
	registry := NewRegistry()
	registry.Register(contracts.CommandPrepareReleaseWorkspace, HandlerFunc(func(_ context.Context, _ contracts.CommandEnvelope) Result {
		return Done("workspace prepared")
	}))

	dispatcher := New(slog.New(slog.NewTextHandler(io.Discard, nil)), registry, writer)
	dispatcher.now = func() time.Time {
		return time.Date(2026, 3, 31, 9, 30, 0, 0, time.UTC)
	}

	err := dispatcher.Dispatch(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_1",
		CorrelationID: "corr_1",
		AgentID:       "agt_1",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    dispatcher.now(),
	})
	if err != nil {
		t.Fatalf("dispatch command: %v", err)
	}

	if len(writer.acks) != 2 {
		t.Fatalf("expected 2 ack envelopes, got %d", len(writer.acks))
	}
	if writer.acks[0].Status != contracts.CommandAckAccepted {
		t.Fatalf("expected first ack to be accepted, got %q", writer.acks[0].Status)
	}
	if writer.acks[1].Status != contracts.CommandAckDone {
		t.Fatalf("expected second ack to be done, got %q", writer.acks[1].Status)
	}
	if len(writer.nacks) != 0 || len(writer.errors) != 0 {
		t.Fatal("expected no nack or error envelopes for successful dispatch")
	}
}

func TestDispatchRejectsUnknownCommandWithNack(t *testing.T) {
	writer := &fakeWriter{}
	dispatcher := New(slog.New(slog.NewTextHandler(io.Discard, nil)), NewRegistry(), writer)

	err := dispatcher.Dispatch(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_1",
		CorrelationID: "corr_1",
		AgentID:       "agt_1",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("dispatch command: %v", err)
	}

	if len(writer.nacks) != 1 {
		t.Fatalf("expected 1 nack envelope, got %d", len(writer.nacks))
	}
	if writer.nacks[0].Type != contracts.NackEnvelopeType {
		t.Fatalf("expected nack type %q, got %q", contracts.NackEnvelopeType, writer.nacks[0].Type)
	}
	if len(writer.acks) != 0 || len(writer.errors) != 0 {
		t.Fatal("expected no ack or error envelopes for unknown command")
	}
}

func TestDispatchMapsRetryableAndNonRetryableErrors(t *testing.T) {
	writer := &fakeWriter{}
	registry := NewRegistry()
	registry.Register(contracts.CommandRenderGatewayConfig, HandlerFunc(func(_ context.Context, envelope contracts.CommandEnvelope) Result {
		if envelope.RequestID == "retryable" {
			return Retryable("gateway_reload_timeout", "gateway reload timed out", map[string]any{"attempt": 1})
		}
		return NonRetryable("gateway_invalid_config", "gateway config is invalid", map[string]any{"path": "/etc/caddy/Caddyfile"})
	}))

	dispatcher := New(slog.New(slog.NewTextHandler(io.Discard, nil)), registry, writer)

	if err := dispatcher.Dispatch(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRenderGatewayConfig,
		RequestID:     "retryable",
		CorrelationID: "corr_retry",
		AgentID:       "agt_1",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("dispatch retryable command: %v", err)
	}

	if err := dispatcher.Dispatch(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRenderGatewayConfig,
		RequestID:     "non_retryable",
		CorrelationID: "corr_non_retry",
		AgentID:       "agt_1",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("dispatch non-retryable command: %v", err)
	}

	if len(writer.errors) != 2 {
		t.Fatalf("expected 2 error envelopes, got %d", len(writer.errors))
	}
	if !writer.errors[0].Retryable {
		t.Fatal("expected first error envelope to be retryable")
	}
	if writer.errors[1].Retryable {
		t.Fatal("expected second error envelope to be non-retryable")
	}
}

func TestNewDefaultRegistryRegistersMinimumCommandSet(t *testing.T) {
	registry := NewDefaultRegistry()

	for _, command := range contracts.MinimumCommandSet {
		if _, ok := registry.Resolve(command); !ok {
			t.Fatalf("expected default registry to include %q", command)
		}
	}
}

type fakeWriter struct {
	mu     sync.Mutex
	acks   []contracts.CommandAckEnvelope
	nacks  []contracts.CommandNackEnvelope
	errors []contracts.CommandErrorEnvelope
}

func (w *fakeWriter) SendCommandAck(_ context.Context, envelope contracts.CommandAckEnvelope) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.acks = append(w.acks, envelope)
	return nil
}

func (w *fakeWriter) SendCommandNack(_ context.Context, envelope contracts.CommandNackEnvelope) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nacks = append(w.nacks, envelope)
	return nil
}

func (w *fakeWriter) SendCommandError(_ context.Context, envelope contracts.CommandErrorEnvelope) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.errors = append(w.errors, envelope)
	return nil
}
