package enroll

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/control"
	"lazyops-agent/internal/state"
)

type Service struct {
	store         *state.Store
	client        control.Client
	logger        *slog.Logger
	encryptionKey string
}

func New(store *state.Store, client control.Client, logger *slog.Logger, encryptionKey string) *Service {
	return &Service{
		store:         store,
		client:        client,
		logger:        logger,
		encryptionKey: strings.TrimSpace(encryptionKey),
	}
}

func (s *Service) Enroll(
	ctx context.Context,
	bootstrapToken string,
	machine contracts.MachineInfo,
	capabilities contracts.CapabilityReportPayload,
	runtimeMode contracts.RuntimeMode,
	agentKind contracts.AgentKind,
) (*contracts.EnrollAgentResponse, error) {
	if strings.TrimSpace(bootstrapToken) == "" {
		return nil, fmt.Errorf("bootstrap token is required")
	}
	if s.encryptionKey == "" {
		return nil, fmt.Errorf("state encryption key is required for enrollment")
	}

	response, err := s.client.Enroll(ctx, contracts.EnrollAgentRequest{
		BootstrapToken: bootstrapToken,
		RuntimeMode:    runtimeMode,
		AgentKind:      agentKind,
		Machine:        machine,
		Capabilities:   capabilities,
	})
	if err != nil {
		return nil, err
	}
	if response.AgentID == "" || response.AgentToken == "" {
		return nil, fmt.Errorf("enroll response missing agent credentials")
	}

	encryptedToken, err := state.EncryptSecret(response.AgentToken, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt agent token: %w", err)
	}

	agentFingerprint := state.Fingerprint(response.AgentToken)
	bootstrapFingerprint := state.Fingerprint(bootstrapToken)
	bootstrapToken = ""

	if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
		local.Metadata.AgentID = response.AgentID
		local.Metadata.AgentKind = agentKind
		local.Metadata.RuntimeMode = runtimeMode
		local.Enrollment.Enrolled = true
		local.Enrollment.LastEnrollmentAt = time.Now().UTC()
		local.Enrollment.TokenReference = agentFingerprint
		local.Enrollment.EncryptedAgentToken = encryptedToken
		local.Enrollment.BootstrapTokenFingerprint = bootstrapFingerprint
		local.Enrollment.BootstrapTokenUsed = true
		local.Enrollment.LastBootstrapAt = time.Now().UTC()
		return nil
	}); err != nil {
		return nil, fmt.Errorf("persist enrollment state: %w", err)
	}

	s.logger.Info("agent enrollment completed",
		"agent_id", response.AgentID,
		"token_reference", agentFingerprint,
		"bootstrap_token", bootstrapFingerprint,
	)

	return &response, nil
}

func (s *Service) LoadAgentToken(ctx context.Context) (string, error) {
	current, err := s.store.Load(ctx)
	if err != nil {
		return "", err
	}
	if !current.Enrollment.Enrolled {
		return "", fmt.Errorf("agent is not enrolled")
	}
	if current.Enrollment.EncryptedAgentToken == "" {
		return "", fmt.Errorf("encrypted agent token is missing")
	}
	return state.DecryptSecret(current.Enrollment.EncryptedAgentToken, s.encryptionKey)
}
