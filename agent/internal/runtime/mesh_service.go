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

type MeshService struct {
	logger  *slog.Logger
	store   *state.Store
	manager *MeshManager
	now     func() time.Time
}

func NewMeshService(logger *slog.Logger, store *state.Store, manager *MeshManager) *MeshService {
	return &MeshService{
		logger:  logger,
		store:   store,
		manager: manager,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *MeshService) Register(registry *dispatcher.Registry) {
	if registry == nil || s.manager == nil {
		return
	}
	registry.Register(contracts.CommandEnsureMeshPeer, dispatcher.HandlerFunc(s.handleEnsureMeshPeer))
	registry.Register(contracts.CommandSyncOverlayRoutes, dispatcher.HandlerFunc(s.handleSyncOverlayRoutes))
}

func (s *MeshService) handleEnsureMeshPeer(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.EnsureMeshPeerPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_ensure_mesh_peer_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	result, err := s.manager.EnsurePeer(ctx, payload)
	if err != nil {
		return dispatchOperationError(
			err,
			"ensure_mesh_peer_failed",
			map[string]any{
				"project_id": payload.ProjectID,
				"binding_id": payload.BindingID,
				"peer_ref":   payload.PeerRef,
			},
		)
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			local.RevisionCache.LastPolicyAction = string(contracts.CommandEnsureMeshPeer)
			local.RevisionCache.UpdatedAt = s.now()
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_mesh_peer_state_failed", fmt.Sprintf("mesh peer ensure completed but local state update failed: %v", err), map[string]any{
				"project_id": payload.ProjectID,
				"binding_id": payload.BindingID,
				"peer_ref":   payload.PeerRef,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("mesh peer ensure completed",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
			"provider", result.Provider,
			"ensured_peers", len(result.EnsuredPeerRefs),
			"removed_peers", len(result.RemovedPeerRefs),
		)
	}

	return dispatcher.Done(result.Summary)
}

func (s *MeshService) handleSyncOverlayRoutes(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.SyncOverlayRoutesPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_sync_overlay_routes_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	result, err := s.manager.SyncOverlayRoutes(ctx, payload)
	if err != nil {
		return dispatchOperationError(
			err,
			"sync_overlay_routes_failed",
			map[string]any{
				"project_id": payload.ProjectID,
				"binding_id": payload.BindingID,
			},
		)
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			local.RevisionCache.LastPolicyAction = string(contracts.CommandSyncOverlayRoutes)
			local.RevisionCache.UpdatedAt = s.now()
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_overlay_route_state_failed", fmt.Sprintf("overlay route sync completed but local state update failed: %v", err), map[string]any{
				"project_id": payload.ProjectID,
				"binding_id": payload.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("overlay route sync completed",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
			"provider", result.Provider,
			"verified_routes", result.VerifiedRoutes,
			"degraded_routes", result.DegradedRoutes,
			"blocked_routes", result.BlockedRoutes,
		)
	}

	return dispatcher.Done(result.Summary)
}
