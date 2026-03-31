package reporting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/state"
)

var capabilityFieldOrder = []string{
	"agent_kind",
	"runtime_mode",
	"control_channel",
	"network",
	"gateway",
	"sidecar",
	"mesh",
	"telemetry",
	"node",
	"performance_targets",
	"additional_capabilities",
}

type Reporter struct {
	logger            *slog.Logger
	heartbeatInterval time.Duration
	now               func() time.Time
}

func New(logger *slog.Logger, heartbeatInterval time.Duration) *Reporter {
	return &Reporter{
		logger:            logger,
		heartbeatInterval: heartbeatInterval,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (r *Reporter) ReconcileCapabilitySnapshot(snapshot *state.CapabilitySnapshotState, payload contracts.CapabilityReportPayload) error {
	if snapshot == nil {
		return fmt.Errorf("capability snapshot is required")
	}

	fingerprint, err := capabilityFingerprint(payload)
	if err != nil {
		return err
	}

	now := r.now()
	snapshot.LastComputedAt = now
	snapshot.Payload = payload

	if snapshot.Version == 0 {
		snapshot.Version = 1
	}

	if snapshot.Fingerprint == fingerprint {
		snapshot.Fingerprint = fingerprint
		return nil
	}

	if snapshot.Fingerprint != "" {
		snapshot.Version++
	}
	snapshot.Fingerprint = fingerprint
	return nil
}

func (r *Reporter) MarkCapabilitiesReported(snapshot *state.CapabilitySnapshotState) {
	if snapshot == nil {
		return
	}

	now := r.now()
	snapshot.LastReportedAt = now
	snapshot.LastReportedFingerprint = snapshot.Fingerprint
	snapshot.LastReportedVersion = snapshot.Version
	snapshot.LastReportedPayload = snapshot.Payload
}

func (r *Reporter) EvaluateHealth(local *state.AgentLocalState) state.HealthSnapshotState {
	now := r.now()
	if local == nil {
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthOffline,
			Summary:   "agent state is unavailable",
			UpdatedAt: now,
		}
	}

	switch local.Metadata.CurrentState {
	case contracts.AgentStateDisconnected:
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthOffline,
			Summary:   "control session disconnected",
			UpdatedAt: now,
		}
	case contracts.AgentStateDegraded:
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthDegraded,
			Summary:   "local runtime marked degraded",
			UpdatedAt: now,
		}
	case contracts.AgentStateReconciling:
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthBusy,
			Summary:   "agent is busy reconciling a revision",
			UpdatedAt: now,
		}
	case contracts.AgentStateReporting:
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthBusy,
			Summary:   "agent is busy reporting telemetry",
			UpdatedAt: now,
		}
	}

	if local.Metadata.AgentID == "" {
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthDegraded,
			Summary:   "agent identity not initialized",
			UpdatedAt: now,
		}
	}
	if local.Enrollment.SessionID == "" && local.Metadata.CurrentState == contracts.AgentStateConnected {
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthDegraded,
			Summary:   "control session ID is missing",
			UpdatedAt: now,
		}
	}
	if local.CapabilitySnapshot.Fingerprint == "" {
		return state.HealthSnapshotState{
			Status:    contracts.AgentHealthDegraded,
			Summary:   "capability snapshot missing",
			UpdatedAt: now,
		}
	}

	return state.HealthSnapshotState{
		Status:    contracts.AgentHealthOnline,
		Summary:   "agent session healthy",
		UpdatedAt: now,
	}
}

func (r *Reporter) BuildHeartbeat(local *state.AgentLocalState) (contracts.HeartbeatPayload, state.HealthSnapshotState, error) {
	if local == nil {
		return contracts.HeartbeatPayload{}, state.HealthSnapshotState{}, fmt.Errorf("agent state is required")
	}
	if local.CapabilitySnapshot.Fingerprint == "" {
		return contracts.HeartbeatPayload{}, state.HealthSnapshotState{}, fmt.Errorf("capability fingerprint is required before heartbeat")
	}

	health := r.EvaluateHealth(local)
	update, err := buildCapabilityUpdate(local.CapabilitySnapshot, r.now())
	if err != nil {
		return contracts.HeartbeatPayload{}, state.HealthSnapshotState{}, err
	}

	uptimeSeconds := int64(0)
	if !local.Metadata.LastStartedAt.IsZero() {
		uptimeSeconds = int64(r.now().Sub(local.Metadata.LastStartedAt).Seconds())
	}

	heartbeat := contracts.HeartbeatPayload{
		AgentID:          local.Metadata.AgentID,
		SessionID:        local.Enrollment.SessionID,
		State:            local.Metadata.CurrentState,
		HealthStatus:     health.Status,
		HealthSummary:    health.Summary,
		RuntimeMode:      local.Metadata.RuntimeMode,
		AgentKind:        local.Metadata.AgentKind,
		SentAt:           r.now(),
		UptimeSeconds:    uptimeSeconds,
		CapabilityHash:   local.CapabilitySnapshot.Fingerprint,
		CapabilityUpdate: &update,
	}
	if update.Mode == contracts.CapabilityUpdateFull {
		heartbeat.Capabilities = local.CapabilitySnapshot.Payload
	}
	return heartbeat, health, nil
}

func buildCapabilityUpdate(snapshot state.CapabilitySnapshotState, sentAt time.Time) (contracts.CapabilityDiffUpdatePayload, error) {
	if snapshot.Fingerprint == "" {
		return contracts.CapabilityDiffUpdatePayload{}, fmt.Errorf("capability fingerprint is required")
	}
	if snapshot.Version == 0 {
		return contracts.CapabilityDiffUpdatePayload{}, fmt.Errorf("capability version is required")
	}

	if snapshot.LastReportedFingerprint == "" || snapshot.LastReportedVersion == 0 {
		full := snapshot.Payload
		return contracts.CapabilityDiffUpdatePayload{
			Mode:        contracts.CapabilityUpdateFull,
			Version:     snapshot.Version,
			CurrentHash: snapshot.Fingerprint,
			Full:        &full,
			SentAt:      sentAt,
		}, nil
	}

	if snapshot.LastReportedFingerprint == snapshot.Fingerprint {
		return contracts.CapabilityDiffUpdatePayload{
			Mode:        contracts.CapabilityUpdateUnchanged,
			Version:     snapshot.Version,
			BaseVersion: snapshot.LastReportedVersion,
			CurrentHash: snapshot.Fingerprint,
			BaseHash:    snapshot.LastReportedFingerprint,
			SentAt:      sentAt,
		}, nil
	}

	changedFields, changed, removedFields, err := diffCapabilities(snapshot.LastReportedPayload, snapshot.Payload)
	if err != nil {
		return contracts.CapabilityDiffUpdatePayload{}, err
	}

	return contracts.CapabilityDiffUpdatePayload{
		Mode:          contracts.CapabilityUpdateDiff,
		Version:       snapshot.Version,
		BaseVersion:   snapshot.LastReportedVersion,
		CurrentHash:   snapshot.Fingerprint,
		BaseHash:      snapshot.LastReportedFingerprint,
		ChangedFields: changedFields,
		Changed:       changed,
		RemovedFields: removedFields,
		SentAt:        sentAt,
	}, nil
}

func capabilityFingerprint(payload contracts.CapabilityReportPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return state.Fingerprint(string(raw)), nil
}

func diffCapabilities(previous, current contracts.CapabilityReportPayload) ([]string, map[string]json.RawMessage, []string, error) {
	previousSections, err := capabilitySections(previous)
	if err != nil {
		return nil, nil, nil, err
	}
	currentSections, err := capabilitySections(current)
	if err != nil {
		return nil, nil, nil, err
	}

	changedFields := make([]string, 0)
	changed := make(map[string]json.RawMessage)
	removedFields := make([]string, 0)

	for _, field := range capabilityFieldOrder {
		previousRaw := previousSections[field]
		currentRaw := currentSections[field]
		if bytes.Equal(previousRaw, currentRaw) {
			continue
		}
		if bytes.Equal(currentRaw, []byte("null")) {
			removedFields = append(removedFields, field)
			continue
		}
		changedFields = append(changedFields, field)
		changed[field] = currentRaw
	}

	sort.Strings(changedFields)
	sort.Strings(removedFields)
	if len(changed) == 0 {
		changed = nil
	}

	return changedFields, changed, removedFields, nil
}

func capabilitySections(payload contracts.CapabilityReportPayload) (map[string]json.RawMessage, error) {
	sections := map[string]any{
		"agent_kind":              payload.AgentKind,
		"runtime_mode":            payload.RuntimeMode,
		"control_channel":         payload.ControlChannel,
		"network":                 payload.Network,
		"gateway":                 payload.Gateway,
		"sidecar":                 payload.Sidecar,
		"mesh":                    payload.Mesh,
		"telemetry":               payload.Telemetry,
		"node":                    payload.Node,
		"performance_targets":     payload.PerformanceTargets,
		"additional_capabilities": payload.AdditionalCapabilities,
	}

	out := make(map[string]json.RawMessage, len(sections))
	for key, value := range sections {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		out[key] = raw
	}
	return out, nil
}
