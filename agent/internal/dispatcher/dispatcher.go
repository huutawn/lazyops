package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type ResponseWriter interface {
	SendCommandAck(context.Context, contracts.CommandAckEnvelope) error
	SendCommandNack(context.Context, contracts.CommandNackEnvelope) error
	SendCommandError(context.Context, contracts.CommandErrorEnvelope) error
}

type Handler interface {
	Handle(context.Context, contracts.CommandEnvelope) Result
}

type HandlerFunc func(context.Context, contracts.CommandEnvelope) Result

func (f HandlerFunc) Handle(ctx context.Context, envelope contracts.CommandEnvelope) Result {
	return f(ctx, envelope)
}

type Result struct {
	Status  contracts.CommandAckStatus
	Summary string
	Error   *DispatchError
}

type DispatchError struct {
	Code      string
	Message   string
	Retryable bool
	Details   map[string]any
}

func (e *DispatchError) Error() string {
	return e.Message
}

func Done(summary string) Result {
	return Result{
		Status:  contracts.CommandAckDone,
		Summary: summary,
	}
}

func Retryable(code, message string, details map[string]any) Result {
	return Result{
		Error: &DispatchError{
			Code:      code,
			Message:   message,
			Retryable: true,
			Details:   details,
		},
	}
}

func NonRetryable(code, message string, details map[string]any) Result {
	return Result{
		Error: &DispatchError{
			Code:      code,
			Message:   message,
			Retryable: false,
			Details:   details,
		},
	}
}

type Registry struct {
	mu       sync.RWMutex
	handlers map[contracts.CommandType]Handler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[contracts.CommandType]Handler),
	}
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	for _, command := range contracts.MinimumCommandSet {
		spec := contracts.CommandHandlerBindings[command]
		specCopy := spec
		registry.Register(command, HandlerFunc(func(_ context.Context, _ contracts.CommandEnvelope) Result {
			return NonRetryable("command_not_implemented", "handler is registered but implementation is still pending", map[string]any{
				"module":      specCopy.Module,
				"handler_key": specCopy.HandlerKey,
				"command":     specCopy.Command,
			})
		}))
	}
	return registry
}

func (r *Registry) Register(command contracts.CommandType, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[command] = handler
}

func (r *Registry) Resolve(command contracts.CommandType) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[command]
	return handler, ok
}

type CommandDispatcher struct {
	logger   *slog.Logger
	registry *Registry
	writer   ResponseWriter
	now      func() time.Time
}

func New(logger *slog.Logger, registry *Registry, writer ResponseWriter) *CommandDispatcher {
	if registry == nil {
		registry = NewDefaultRegistry()
	}
	return &CommandDispatcher{
		logger:   logger,
		registry: registry,
		writer:   writer,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (d *CommandDispatcher) Dispatch(ctx context.Context, envelope contracts.CommandEnvelope) error {
	log := d.logger
	if log != nil {
		log = log.With(
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"command_type", envelope.Type,
		)
	}

	if nack := d.validateEnvelope(envelope); nack != nil {
		if log != nil {
			log.Warn("command envelope rejected", "code", nack.Code, "message", nack.Message)
		}
		return d.writer.SendCommandNack(ctx, *nack)
	}

	handler, ok := d.registry.Resolve(envelope.Type)
	if !ok {
		nack := d.newNack(envelope, "command_not_supported", "no handler is registered for the requested command", map[string]any{
			"command_type": envelope.Type,
		})
		if log != nil {
			log.Warn("command envelope rejected", "code", nack.Code, "message", nack.Message)
		}
		return d.writer.SendCommandNack(ctx, nack)
	}

	if err := d.writer.SendCommandAck(ctx, d.newAck(envelope, contracts.CommandAckAccepted, "command accepted by dispatcher")); err != nil {
		return err
	}

	result := handler.Handle(ctx, envelope)
	if result.Error != nil {
		if log != nil {
			log.Warn("command handler failed",
				"code", result.Error.Code,
				"retryable", result.Error.Retryable,
				"message", result.Error.Message,
			)
		}
		return d.writer.SendCommandError(ctx, d.newError(envelope, *result.Error))
	}

	status := result.Status
	if status == "" {
		status = contracts.CommandAckDone
	}
	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		summary = "command completed"
	}

	if log != nil {
		log.Info("command handler completed", "status", status, "summary", summary)
	}
	return d.writer.SendCommandAck(ctx, d.newAck(envelope, status, summary))
}

func (d *CommandDispatcher) validateEnvelope(envelope contracts.CommandEnvelope) *contracts.CommandNackEnvelope {
	if envelope.Source != contracts.EnvelopeSourceBackend {
		nack := d.newNack(envelope, "invalid_source", "dispatcher only accepts backend command envelopes", map[string]any{
			"source": envelope.Source,
		})
		return &nack
	}
	if strings.TrimSpace(envelope.RequestID) == "" {
		nack := d.newNack(envelope, "missing_request_id", "command envelope is missing request_id", nil)
		return &nack
	}
	if strings.TrimSpace(envelope.CorrelationID) == "" {
		nack := d.newNack(envelope, "missing_correlation_id", "command envelope is missing correlation_id", nil)
		return &nack
	}
	if strings.TrimSpace(string(envelope.Type)) == "" {
		nack := d.newNack(envelope, "missing_command_type", "command envelope is missing type", nil)
		return &nack
	}
	return nil
}

func (d *CommandDispatcher) newAck(envelope contracts.CommandEnvelope, status contracts.CommandAckStatus, summary string) contracts.CommandAckEnvelope {
	return contracts.CommandAckEnvelope{
		Type:          contracts.AckEnvelopeType,
		RequestID:     envelope.RequestID,
		CorrelationID: envelope.CorrelationID,
		AgentID:       envelope.AgentID,
		CommandType:   envelope.Type,
		Status:        status,
		Source:        contracts.EnvelopeSourceAgent,
		OccurredAt:    d.now(),
		Summary:       summary,
	}
}

func (d *CommandDispatcher) newNack(envelope contracts.CommandEnvelope, code, message string, details map[string]any) contracts.CommandNackEnvelope {
	return contracts.CommandNackEnvelope{
		Type:          contracts.NackEnvelopeType,
		RequestID:     envelope.RequestID,
		CorrelationID: envelope.CorrelationID,
		AgentID:       envelope.AgentID,
		CommandType:   envelope.Type,
		Code:          code,
		Message:       message,
		Source:        contracts.EnvelopeSourceAgent,
		OccurredAt:    d.now(),
		Details:       details,
	}
}

func (d *CommandDispatcher) newError(envelope contracts.CommandEnvelope, dispatchErr DispatchError) contracts.CommandErrorEnvelope {
	code := strings.TrimSpace(dispatchErr.Code)
	if code == "" {
		code = "command_handler_failed"
	}
	message := strings.TrimSpace(dispatchErr.Message)
	if message == "" {
		message = "command handler failed"
	}

	return contracts.CommandErrorEnvelope{
		Type:          contracts.ErrorEnvelopeType,
		RequestID:     envelope.RequestID,
		CorrelationID: envelope.CorrelationID,
		AgentID:       envelope.AgentID,
		CommandType:   envelope.Type,
		Code:          code,
		Message:       message,
		Retryable:     dispatchErr.Retryable,
		Source:        contracts.EnvelopeSourceAgent,
		OccurredAt:    d.now(),
		Details:       dispatchErr.Details,
	}
}

func (d *CommandDispatcher) Handler() func(context.Context, contracts.CommandEnvelope) {
	return func(ctx context.Context, envelope contracts.CommandEnvelope) {
		if err := d.Dispatch(ctx, envelope); err != nil && d.logger != nil {
			d.logger.Error("dispatch command envelope", "error", err, "command_type", envelope.Type)
		}
	}
}

func UnknownCommandResult(command contracts.CommandType) Result {
	return NonRetryable("command_not_supported", fmt.Sprintf("command %q is not registered", command), map[string]any{
		"command_type": command,
	})
}
