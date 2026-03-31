package control

import (
	"context"

	"lazyops-agent/internal/contracts"
)

type Client interface {
	Connect(context.Context, contracts.SessionAuthPayload) error
	SendHandshake(context.Context, contracts.AgentHandshakePayload) error
	SendHeartbeat(context.Context, contracts.HeartbeatPayload) error
	Close(context.Context) error
	Transcript() []contracts.CommandEnvelope
}
