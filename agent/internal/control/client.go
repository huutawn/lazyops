package control

import (
	"context"

	"lazyops-agent/internal/contracts"
)

type CommandHandler func(context.Context, contracts.CommandEnvelope)

type Client interface {
	Enroll(context.Context, contracts.EnrollAgentRequest) (contracts.EnrollAgentResponse, error)
	Connect(context.Context, contracts.SessionAuthPayload) error
	RegisterCommandHandler(CommandHandler)
	SendHandshake(context.Context, contracts.AgentHandshakePayload) error
	SendHeartbeat(context.Context, contracts.HeartbeatPayload) error
	SendCommandAck(context.Context, contracts.CommandAckEnvelope) error
	SendCommandNack(context.Context, contracts.CommandNackEnvelope) error
	SendCommandError(context.Context, contracts.CommandErrorEnvelope) error
	Close(context.Context) error
	Transcript() []contracts.CommandEnvelope
}
