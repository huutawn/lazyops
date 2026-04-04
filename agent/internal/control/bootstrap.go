package control

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

const (
	handshakeEnvelopeType    = contracts.CommandType("agent.handshake")
	heartbeatEnvelopeType    = contracts.CommandType("agent.heartbeat")
	traceSummaryEnvelopeType = contracts.CommandType("agent.trace_summary")
	logBatchEnvelopeType     = contracts.CommandType("agent.log_batch")
	metricRollupEnvelopeType = contracts.CommandType("agent.metric_rollup")
	topologyEnvelopeType     = contracts.CommandType("agent.topology")
	incidentEnvelopeType     = contracts.CommandType("agent.incident")
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

type bootstrapRegistry struct {
	mu      sync.Mutex
	records map[string]BootstrapTokenRecord
}

func newDefaultBootstrapRegistry() *bootstrapRegistry {
	return &bootstrapRegistry{records: defaultBootstrapRegistry()}
}

func (r *bootstrapRegistry) Enroll(_ context.Context, req contracts.EnrollAgentRequest) (contracts.EnrollAgentResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[req.BootstrapToken]
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
	r.records[req.BootstrapToken] = record

	return contracts.EnrollAgentResponse{
		AgentID:    record.AgentID,
		AgentToken: record.AgentToken,
		IssuedAt:   time.Now().UTC(),
	}, nil
}

func buildEnvelope(messageType contracts.CommandType, agentID string, payload any) (contracts.CommandEnvelope, []byte, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return contracts.CommandEnvelope{}, nil, err
	}

	envelope := contracts.CommandEnvelope{
		Type:          messageType,
		RequestID:     "",
		CorrelationID: "",
		AgentID:       agentID,
		ProjectID:     "",
		RevisionID:    "",
		Source:        contracts.EnvelopeSourceAgent,
		OccurredAt:    time.Now().UTC(),
		Payload:       rawPayload,
	}

	rawEnvelope, err := json.Marshal(envelope)
	if err != nil {
		return contracts.CommandEnvelope{}, nil, err
	}
	return envelope, rawEnvelope, nil
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
