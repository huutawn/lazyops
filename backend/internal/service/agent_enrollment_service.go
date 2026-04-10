package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var (
	ErrBootstrapTokenUnknown      = errors.New("bootstrap token unknown")
	ErrBootstrapTokenExpired      = errors.New("bootstrap token expired")
	ErrBootstrapTokenReused       = errors.New("bootstrap token reused")
	ErrBootstrapOwnershipMismatch = errors.New("bootstrap ownership mismatch")
)

type AgentEnrollmentService struct {
	agents      AgentStore
	instances   InstanceStore
	bootstrap   BootstrapTokenStore
	agentTokens AgentTokenStore
	enrollment  config.EnrollmentConfig
}

func NewAgentEnrollmentService(
	agents AgentStore,
	instances InstanceStore,
	bootstrap BootstrapTokenStore,
	agentTokens AgentTokenStore,
	enrollment config.EnrollmentConfig,
) *AgentEnrollmentService {
	return &AgentEnrollmentService{
		agents:      agents,
		instances:   instances,
		bootstrap:   bootstrap,
		agentTokens: agentTokens,
		enrollment:  enrollment,
	}
}

func (s *AgentEnrollmentService) Enroll(cmd AgentEnrollmentCommand) (*AgentEnrollmentResult, error) {
	bootstrapToken := strings.TrimSpace(cmd.BootstrapToken)
	if bootstrapToken == "" {
		return nil, ErrInvalidInput
	}
	if err := validateInstanceEnrollmentMode(cmd.RuntimeMode, cmd.AgentKind); err != nil {
		return nil, err
	}

	record, err := s.bootstrap.GetByHash(hashOpaqueToken(bootstrapToken))
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrBootstrapTokenUnknown
	}

	now := time.Now().UTC()
	if record.UsedAt != nil {
		return nil, ErrBootstrapTokenReused
	}
	if now.After(record.ExpiresAt) {
		return nil, ErrBootstrapTokenExpired
	}

	instance, err := s.instances.GetByID(record.InstanceID)
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.UserID != record.UserID {
		return nil, ErrBootstrapOwnershipMismatch
	}

	if err := validateEnrollmentMachineOwnership(*instance, cmd.Machine); err != nil {
		return nil, err
	}

	capabilitiesJSON, err := marshalCapabilitiesJSON(cmd.Capabilities)
	if err != nil {
		return nil, err
	}

	agentName := normalizeEnrollmentAgentName(cmd.Machine.Hostname, instance.Name)
	agent, err := s.resolveOrCreateAgent(instance, agentName, now)
	if err != nil {
		return nil, err
	}

	rawToken, tokenPrefix, err := newOpaqueAgentToken()
	if err != nil {
		return nil, err
	}

	if err := s.agentTokens.RevokeByAgent(agent.AgentID, now); err != nil {
		return nil, err
	}

	expiresAt := now.Add(s.agentTokenTTL())
	token := &models.AgentToken{
		ID:          utils.NewPrefixedID("atok"),
		UserID:      instance.UserID,
		InstanceID:  instance.ID,
		AgentID:     agent.AgentID,
		TokenHash:   hashOpaqueToken(rawToken),
		TokenPrefix: tokenPrefix,
		ExpiresAt:   &expiresAt,
	}
	if err := s.agentTokens.Create(token); err != nil {
		return nil, err
	}

	if _, err := s.instances.UpdateAgentState(instance.ID, agent.AgentID, "online", &capabilitiesJSON, now); err != nil {
		return nil, err
	}

	if err := s.bootstrap.MarkUsed(record.ID, now); err != nil {
		return nil, err
	}

	return &AgentEnrollmentResult{
		AgentID:    agent.AgentID,
		AgentToken: rawToken,
		InstanceID: instance.ID,
		IssuedAt:   now,
		ExpiresAt:  &expiresAt,
	}, nil
}

func (s *AgentEnrollmentService) Heartbeat(cmd AgentHeartbeatCommand) (*AgentHeartbeatResult, error) {
	userID := strings.TrimSpace(cmd.UserID)
	agentID := strings.TrimSpace(cmd.AgentID)
	instanceID := strings.TrimSpace(cmd.InstanceID)
	if userID == "" || agentID == "" || instanceID == "" {
		return nil, ErrInvalidInput
	}
	if err := validateInstanceEnrollmentMode(cmd.RuntimeMode, cmd.AgentKind); err != nil {
		return nil, err
	}

	at := cmd.SentAt
	if at.IsZero() {
		at = time.Now().UTC()
	}

	agentStatus := normalizeAgentHeartbeatStatus(cmd.HealthStatus, cmd.State)
	agent, err := s.agents.UpdateStatusForUser(userID, agentID, "", agentStatus, at)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}

	instanceStatus := normalizeInstanceHeartbeatStatus(cmd.HealthStatus, cmd.State)
	var runtimeCapabilitiesJSON *string
	if cmd.Capabilities != nil {
		encoded, err := marshalCapabilitiesJSON(cmd.Capabilities)
		if err != nil {
			return nil, err
		}
		runtimeCapabilitiesJSON = &encoded
	}

	instance, err := s.instances.UpdateAgentState(instanceID, agentID, instanceStatus, runtimeCapabilitiesJSON, at)
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.UserID != userID {
		return nil, ErrBootstrapOwnershipMismatch
	}

	return &AgentHeartbeatResult{
		AgentID:        agentID,
		InstanceID:     instanceID,
		AgentStatus:    agentStatus,
		InstanceStatus: instanceStatus,
		ReceivedAt:     time.Now().UTC(),
	}, nil
}

func (s *AgentEnrollmentService) resolveOrCreateAgent(instance *models.Instance, agentName string, at time.Time) (*models.Agent, error) {
	if instance.AgentID != nil && strings.TrimSpace(*instance.AgentID) != "" {
		agent, err := s.agents.GetByAgentIDForUser(instance.UserID, strings.TrimSpace(*instance.AgentID))
		if err != nil {
			return nil, err
		}
		if agent != nil {
			return s.agents.UpdateStatusForUser(instance.UserID, agent.AgentID, agentName, "online", at)
		}
	}

	agentIdentifier := utils.NewPrefixedID("agt")
	agent := &models.Agent{
		ID:         agentIdentifier,
		UserID:     instance.UserID,
		AgentID:    agentIdentifier,
		Name:       agentName,
		Status:     "online",
		LastSeenAt: &at,
	}
	if err := s.agents.Create(agent); err != nil {
		return nil, err
	}

	return agent, nil
}

func validateInstanceEnrollmentMode(runtimeMode, agentKind string) error {
	runtimeMode = strings.TrimSpace(runtimeMode)
	agentKind = strings.TrimSpace(agentKind)
	if runtimeMode == "" || agentKind == "" {
		return ErrInvalidInput
	}

	if agentKind != "instance_agent" {
		return ErrInvalidInput
	}
	if runtimeMode != "standalone" && runtimeMode != "distributed-mesh" {
		return ErrInvalidInput
	}

	return nil
}

func validateEnrollmentMachineOwnership(instance models.Instance, machine AgentMachineInfo) error {
	if len(machine.IPs) == 0 {
		return ErrBootstrapOwnershipMismatch
	}

	allowed := make(map[string]struct{}, 2)
	if instance.PublicIP != nil && *instance.PublicIP != "" {
		allowed[*instance.PublicIP] = struct{}{}
	}
	if instance.PrivateIP != nil && *instance.PrivateIP != "" {
		allowed[*instance.PrivateIP] = struct{}{}
	}
	if len(allowed) == 0 {
		// Lazy SSH onboarding can intentionally omit instance IP metadata.
		// In that case, single-use bootstrap token possession is the trust anchor.
		return nil
	}

	for _, rawIP := range machine.IPs {
		value := strings.TrimSpace(rawIP)
		if value == "" {
			continue
		}
		ip := net.ParseIP(value)
		if ip == nil {
			return ErrInvalidInput
		}
		if _, ok := allowed[ip.String()]; ok {
			return nil
		}
	}

	return ErrBootstrapOwnershipMismatch
}

func normalizeEnrollmentAgentName(hostname, fallback string) string {
	name := utils.NormalizeSpace(hostname)
	if name == "" {
		name = utils.NormalizeSpace(fallback)
	}
	if name == "" {
		return "LazyOps Agent"
	}
	return name
}

func marshalCapabilitiesJSON(capabilities map[string]any) (string, error) {
	if capabilities == nil {
		capabilities = map[string]any{}
	}

	raw, err := json.Marshal(capabilities)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func normalizeAgentHeartbeatStatus(healthStatus, state string) string {
	switch strings.TrimSpace(strings.ToLower(healthStatus)) {
	case "busy":
		return "busy"
	case "degraded":
		return "error"
	case "offline":
		return "offline"
	case "online":
		return "online"
	}

	switch strings.TrimSpace(strings.ToLower(state)) {
	case "reconciling":
		return "busy"
	case "degraded":
		return "error"
	case "disconnected":
		return "offline"
	default:
		return "online"
	}
}

func normalizeInstanceHeartbeatStatus(healthStatus, state string) string {
	switch strings.TrimSpace(strings.ToLower(healthStatus)) {
	case "degraded":
		return "degraded"
	case "offline":
		return "offline"
	case "busy":
		return "online"
	case "online":
		return "online"
	}

	switch strings.TrimSpace(strings.ToLower(state)) {
	case "degraded":
		return "degraded"
	case "disconnected":
		return "offline"
	default:
		return "online"
	}
}

func (s *AgentEnrollmentService) agentTokenTTL() time.Duration {
	if s.enrollment.AgentTokenTTL <= 0 {
		return 30 * 24 * time.Hour
	}
	return s.enrollment.AgentTokenTTL
}

func newOpaqueAgentToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}

	token := "lop_atok_" + base64.RawURLEncoding.EncodeToString(raw)
	prefix := token
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}

	return token, prefix, nil
}
