package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/dispatcher"
	"lazyops-agent/internal/state"
)

type Service struct {
	logger *slog.Logger
	store  *state.Store
	driver Driver
	now    func() time.Time
}

func NewService(logger *slog.Logger, store *state.Store, driver Driver) *Service {
	return &Service{
		logger: logger,
		store:  store,
		driver: driver,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) Register(registry *dispatcher.Registry) {
	if registry == nil {
		return
	}
	registry.Register(contracts.CommandPrepareReleaseWorkspace, dispatcher.HandlerFunc(s.handlePrepareReleaseWorkspace))
}

func (s *Service) handlePrepareReleaseWorkspace(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_prepare_release_workspace_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	prepared, err := s.driver.PrepareReleaseWorkspace(ctx, runtimeCtx)
	if err != nil {
		return dispatcher.Retryable("prepare_release_workspace_failed", err.Error(), map[string]any{
			"revision_id": runtimeCtx.Revision.RevisionID,
			"binding_id":  runtimeCtx.Binding.BindingID,
		})
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			recordPendingRevision(local, runtimeCtx.Revision.RevisionID, s.now())
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_runtime_workspace_failed", fmt.Sprintf("workspace prepared but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime workspace prepared",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"workspace_root", prepared.Layout.Root,
		)
	}

	return dispatcher.Done("release workspace prepared")
}
