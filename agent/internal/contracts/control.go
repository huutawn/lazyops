package contracts

import (
	"encoding/json"
	"time"
)

const (
	ControlWebSocketPath = "/ws/agents/control"
	AckEnvelopeType      = "command.ack"
	NackEnvelopeType     = "command.nack"
	ErrorEnvelopeType    = "command.error"
)

// CommandEnvelope is the shared transport shell for control-plane commands.
// request_id is unique per command attempt. correlation_id is shared across
// rollout, trace, and incident chains. occurred_at is always UTC in RFC3339
// form when serialized. source identifies the producer of the envelope.
type CommandEnvelope struct {
	Type          CommandType     `json:"type"`
	RequestID     string          `json:"request_id"`
	CorrelationID string          `json:"correlation_id"`
	AgentID       string          `json:"agent_id"`
	ProjectID     string          `json:"project_id,omitempty"`
	RevisionID    string          `json:"revision_id,omitempty"`
	Source        EnvelopeSource  `json:"source"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

type SessionAuthPayload struct {
	AgentID      string      `json:"agent_id"`
	AgentToken   string      `json:"agent_token"`
	SessionID    string      `json:"session_id"`
	RuntimeMode  RuntimeMode `json:"runtime_mode"`
	AgentKind    AgentKind   `json:"agent_kind"`
	HandshakeVer string      `json:"handshake_version"`
	SentAt       time.Time   `json:"sent_at"`
}

type AgentHandshakePayload struct {
	Auth         SessionAuthPayload      `json:"auth"`
	Machine      MachineInfo             `json:"machine"`
	State        AgentState              `json:"state"`
	Capabilities CapabilityReportPayload `json:"capabilities"`
}

type CommandAckStatus string

const (
	CommandAckAccepted CommandAckStatus = "accepted"
	CommandAckRunning  CommandAckStatus = "running"
	CommandAckDone     CommandAckStatus = "done"
)

type CommandAckEnvelope struct {
	Type          string           `json:"type"`
	RequestID     string           `json:"request_id"`
	CorrelationID string           `json:"correlation_id"`
	AgentID       string           `json:"agent_id"`
	CommandType   CommandType      `json:"command_type"`
	Status        CommandAckStatus `json:"status"`
	Source        EnvelopeSource   `json:"source"`
	OccurredAt    time.Time        `json:"occurred_at"`
	Summary       string           `json:"summary,omitempty"`
}

type CommandNackEnvelope struct {
	Type          string         `json:"type"`
	RequestID     string         `json:"request_id"`
	CorrelationID string         `json:"correlation_id"`
	AgentID       string         `json:"agent_id"`
	CommandType   CommandType    `json:"command_type"`
	Code          string         `json:"code"`
	Message       string         `json:"message"`
	Source        EnvelopeSource `json:"source"`
	OccurredAt    time.Time      `json:"occurred_at"`
	Details       map[string]any `json:"details,omitempty"`
}

type CommandErrorEnvelope struct {
	Type          string         `json:"type"`
	RequestID     string         `json:"request_id"`
	CorrelationID string         `json:"correlation_id"`
	AgentID       string         `json:"agent_id"`
	CommandType   CommandType    `json:"command_type"`
	Code          string         `json:"code"`
	Message       string         `json:"message"`
	Retryable     bool           `json:"retryable"`
	Source        EnvelopeSource `json:"source"`
	OccurredAt    time.Time      `json:"occurred_at"`
	Details       map[string]any `json:"details,omitempty"`
}
