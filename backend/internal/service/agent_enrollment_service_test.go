package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
)

type fakeAgentTokenStore struct {
	byID       map[string]*models.AgentToken
	byHash     map[string]*models.AgentToken
	lastUsed   map[string]time.Time
	revokedFor []string
}

func newFakeAgentTokenStore(tokens ...*models.AgentToken) *fakeAgentTokenStore {
	store := &fakeAgentTokenStore{
		byID:     make(map[string]*models.AgentToken),
		byHash:   make(map[string]*models.AgentToken),
		lastUsed: make(map[string]time.Time),
	}

	for _, token := range tokens {
		cloned := *token
		store.byID[cloned.ID] = &cloned
		store.byHash[cloned.TokenHash] = &cloned
	}

	return store
}

func (f *fakeAgentTokenStore) Create(token *models.AgentToken) error {
	cloned := *token
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = time.Now().UTC()
	}
	f.byID[cloned.ID] = &cloned
	f.byHash[cloned.TokenHash] = &cloned
	token.CreatedAt = cloned.CreatedAt
	return nil
}

func (f *fakeAgentTokenStore) GetByHash(tokenHash string) (*models.AgentToken, error) {
	if token, ok := f.byHash[tokenHash]; ok {
		return token, nil
	}
	return nil, nil
}

func (f *fakeAgentTokenStore) TouchLastUsed(tokenID string, at time.Time) error {
	f.lastUsed[tokenID] = at
	if token, ok := f.byID[tokenID]; ok {
		token.LastUsedAt = &at
	}
	return nil
}

func (f *fakeAgentTokenStore) RevokeByAgent(agentID string, at time.Time) error {
	f.revokedFor = append(f.revokedFor, agentID)
	for _, token := range f.byID {
		if token.AgentID == agentID && token.RevokedAt == nil {
			token.RevokedAt = &at
		}
	}
	return nil
}

func testEnrollmentAndAgentTokenConfig() config.EnrollmentConfig {
	return config.EnrollmentConfig{
		BootstrapTokenTTL: 20 * time.Minute,
		AgentTokenTTL:     72 * time.Hour,
	}
}

func TestAgentEnrollmentRejectsExpiredBootstrapToken(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-hcm-1",
		PublicIP:                ptrString("203.0.113.10"),
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	bootstrapStore := newFakeBootstrapTokenStore(&models.BootstrapToken{
		ID:         "boot_1",
		UserID:     "usr_1",
		InstanceID: "inst_1",
		TokenHash:  hashOpaqueToken("lop_boot_expired"),
		ExpiresAt:  time.Now().UTC().Add(-time.Minute),
	})
	agentStore := &fakeAgentStore{}
	agentTokenStore := newFakeAgentTokenStore()
	service := NewAgentEnrollmentService(agentStore, instanceStore, bootstrapStore, agentTokenStore, testEnrollmentAndAgentTokenConfig())

	_, err := service.Enroll(AgentEnrollmentCommand{
		BootstrapToken: "lop_boot_expired",
		RuntimeMode:    "standalone",
		AgentKind:      "instance_agent",
		Machine: AgentMachineInfo{
			Hostname: "edge-hcm-1",
			IPs:      []string{"203.0.113.10"},
		},
		Capabilities: map[string]any{},
	})
	if !errors.Is(err, ErrBootstrapTokenExpired) {
		t.Fatalf("expected ErrBootstrapTokenExpired, got %v", err)
	}
}

func TestAgentEnrollmentRejectsReusedBootstrapToken(t *testing.T) {
	usedAt := time.Now().UTC().Add(-time.Minute)
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-hcm-1",
		PublicIP:                ptrString("203.0.113.10"),
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	bootstrapStore := newFakeBootstrapTokenStore(&models.BootstrapToken{
		ID:         "boot_1",
		UserID:     "usr_1",
		InstanceID: "inst_1",
		TokenHash:  hashOpaqueToken("lop_boot_used"),
		ExpiresAt:  time.Now().UTC().Add(time.Minute),
		UsedAt:     &usedAt,
	})
	agentStore := &fakeAgentStore{}
	agentTokenStore := newFakeAgentTokenStore()
	service := NewAgentEnrollmentService(agentStore, instanceStore, bootstrapStore, agentTokenStore, testEnrollmentAndAgentTokenConfig())

	_, err := service.Enroll(AgentEnrollmentCommand{
		BootstrapToken: "lop_boot_used",
		RuntimeMode:    "standalone",
		AgentKind:      "instance_agent",
		Machine: AgentMachineInfo{
			Hostname: "edge-hcm-1",
			IPs:      []string{"203.0.113.10"},
		},
		Capabilities: map[string]any{},
	})
	if !errors.Is(err, ErrBootstrapTokenReused) {
		t.Fatalf("expected ErrBootstrapTokenReused, got %v", err)
	}
}

func TestAgentEnrollmentRejectsOwnershipMismatch(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-hcm-1",
		PublicIP:                ptrString("203.0.113.10"),
		PrivateIP:               ptrString("10.0.0.5"),
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	bootstrapStore := newFakeBootstrapTokenStore(&models.BootstrapToken{
		ID:         "boot_1",
		UserID:     "usr_1",
		InstanceID: "inst_1",
		TokenHash:  hashOpaqueToken("lop_boot_valid"),
		ExpiresAt:  time.Now().UTC().Add(time.Minute),
	})
	agentStore := &fakeAgentStore{}
	agentTokenStore := newFakeAgentTokenStore()
	service := NewAgentEnrollmentService(agentStore, instanceStore, bootstrapStore, agentTokenStore, testEnrollmentAndAgentTokenConfig())

	_, err := service.Enroll(AgentEnrollmentCommand{
		BootstrapToken: "lop_boot_valid",
		RuntimeMode:    "standalone",
		AgentKind:      "instance_agent",
		Machine: AgentMachineInfo{
			Hostname: "wrong-host",
			IPs:      []string{"198.51.100.22"},
		},
		Capabilities: map[string]any{},
	})
	if !errors.Is(err, ErrBootstrapOwnershipMismatch) {
		t.Fatalf("expected ErrBootstrapOwnershipMismatch, got %v", err)
	}
}

func TestAgentEnrollmentAllowsMissingInstanceIPs(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-hcm-1",
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	bootstrapStore := newFakeBootstrapTokenStore(&models.BootstrapToken{
		ID:         "boot_1",
		UserID:     "usr_1",
		InstanceID: "inst_1",
		TokenHash:  hashOpaqueToken("lop_boot_valid_no_ip"),
		ExpiresAt:  time.Now().UTC().Add(time.Minute),
	})
	agentStore := &fakeAgentStore{}
	agentTokenStore := newFakeAgentTokenStore()
	service := NewAgentEnrollmentService(agentStore, instanceStore, bootstrapStore, agentTokenStore, testEnrollmentAndAgentTokenConfig())

	enrolled, err := service.Enroll(AgentEnrollmentCommand{
		BootstrapToken: "lop_boot_valid_no_ip",
		RuntimeMode:    "standalone",
		AgentKind:      "instance_agent",
		Machine: AgentMachineInfo{
			Hostname: "edge-hcm-1",
			IPs:      []string{"13.212.158.91"},
		},
		Capabilities: map[string]any{},
	})
	if err != nil {
		t.Fatalf("enroll agent without instance ips should succeed: %v", err)
	}
	if !strings.HasPrefix(enrolled.AgentID, "agt_") {
		t.Fatalf("expected enrolled agent id, got %q", enrolled.AgentID)
	}
}

func TestAgentEnrollmentAllowsPrivateOnlyMachineIPsWhenInstanceHasPublicOnly(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-hcm-1",
		PublicIP:                ptrString("13.212.158.91"),
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	bootstrapStore := newFakeBootstrapTokenStore(&models.BootstrapToken{
		ID:         "boot_1",
		UserID:     "usr_1",
		InstanceID: "inst_1",
		TokenHash:  hashOpaqueToken("lop_boot_public_only"),
		ExpiresAt:  time.Now().UTC().Add(time.Minute),
	})
	agentStore := &fakeAgentStore{}
	agentTokenStore := newFakeAgentTokenStore()
	service := NewAgentEnrollmentService(agentStore, instanceStore, bootstrapStore, agentTokenStore, testEnrollmentAndAgentTokenConfig())

	enrolled, err := service.Enroll(AgentEnrollmentCommand{
		BootstrapToken: "lop_boot_public_only",
		RuntimeMode:    "standalone",
		AgentKind:      "instance_agent",
		Machine: AgentMachineInfo{
			Hostname: "ip-172-31-10-34.ap-southeast-1.compute.internal",
			IPs:      []string{"172.31.10.34"},
		},
		Capabilities: map[string]any{},
	})
	if err != nil {
		t.Fatalf("enroll agent with private-only machine IPs should succeed: %v", err)
	}
	if !strings.HasPrefix(enrolled.AgentID, "agt_") {
		t.Fatalf("expected enrolled agent id, got %q", enrolled.AgentID)
	}
}

func TestAgentEnrollmentMarksInstanceOnlineAndHeartbeatUpdatesState(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-hcm-1",
		PublicIP:                ptrString("203.0.113.10"),
		PrivateIP:               ptrString("10.0.0.5"),
		Status:                  "pending_enrollment",
		LabelsJSON:              `{"region":"sg"}`,
		RuntimeCapabilitiesJSON: `{}`,
	})
	bootstrapStore := newFakeBootstrapTokenStore(&models.BootstrapToken{
		ID:         "boot_1",
		UserID:     "usr_1",
		InstanceID: "inst_1",
		TokenHash:  hashOpaqueToken("lop_boot_valid"),
		ExpiresAt:  time.Now().UTC().Add(time.Minute),
	})
	agentStore := &fakeAgentStore{}
	agentTokenStore := newFakeAgentTokenStore()
	service := NewAgentEnrollmentService(agentStore, instanceStore, bootstrapStore, agentTokenStore, testEnrollmentAndAgentTokenConfig())

	enrolled, err := service.Enroll(AgentEnrollmentCommand{
		BootstrapToken: "lop_boot_valid",
		RuntimeMode:    "standalone",
		AgentKind:      "instance_agent",
		Machine: AgentMachineInfo{
			Hostname: "edge-hcm-1",
			OS:       "linux",
			Arch:     "amd64",
			IPs:      []string{"10.0.0.5", "203.0.113.10"},
		},
		Capabilities: map[string]any{
			"runtime_mode": "standalone",
			"telemetry": map[string]any{
				"metric_rollup": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("enroll agent: %v", err)
	}
	if !strings.HasPrefix(enrolled.AgentID, "agt_") {
		t.Fatalf("expected enrolled agent id, got %q", enrolled.AgentID)
	}
	if !strings.HasPrefix(enrolled.AgentToken, "lop_atok_") {
		t.Fatalf("expected agent token prefix, got %q", enrolled.AgentToken)
	}

	instance, err := instanceStore.GetByID("inst_1")
	if err != nil {
		t.Fatalf("reload instance: %v", err)
	}
	if instance.Status != "online" {
		t.Fatalf("expected instance online after enroll, got %q", instance.Status)
	}
	if instance.AgentID == nil || *instance.AgentID != enrolled.AgentID {
		t.Fatalf("expected instance agent binding to %q, got %#v", enrolled.AgentID, instance.AgentID)
	}

	bootstrapRecord, err := bootstrapStore.GetByHash(hashOpaqueToken("lop_boot_valid"))
	if err != nil {
		t.Fatalf("reload bootstrap token: %v", err)
	}
	if bootstrapRecord == nil || bootstrapRecord.UsedAt == nil {
		t.Fatal("expected bootstrap token to be marked used")
	}

	agentTokenRecord, err := agentTokenStore.GetByHash(hashOpaqueToken(enrolled.AgentToken))
	if err != nil {
		t.Fatalf("reload agent token: %v", err)
	}
	if agentTokenRecord == nil {
		t.Fatal("expected issued agent token to be stored by hash")
	}
	if agentTokenRecord.AgentID != enrolled.AgentID {
		t.Fatalf("expected agent token to bind agent %q, got %q", enrolled.AgentID, agentTokenRecord.AgentID)
	}

	sentAt := time.Now().UTC().Add(time.Minute)
	heartbeat, err := service.Heartbeat(AgentHeartbeatCommand{
		UserID:       "usr_1",
		AgentID:      enrolled.AgentID,
		InstanceID:   "inst_1",
		SessionID:    "sess_heartbeat",
		State:        "connected",
		HealthStatus: "online",
		RuntimeMode:  "standalone",
		AgentKind:    "instance_agent",
		SentAt:       sentAt,
		Capabilities: map[string]any{
			"runtime_mode": "standalone",
			"network": map[string]any{
				"outbound_only": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if heartbeat.InstanceStatus != "online" {
		t.Fatalf("expected heartbeat instance status online, got %q", heartbeat.InstanceStatus)
	}
	if heartbeat.AgentStatus != "online" {
		t.Fatalf("expected heartbeat agent status online, got %q", heartbeat.AgentStatus)
	}

	agent, err := agentStore.GetByAgentIDForUser("usr_1", enrolled.AgentID)
	if err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	if agent == nil || agent.LastSeenAt == nil || !agent.LastSeenAt.Equal(sentAt) {
		t.Fatalf("expected agent last seen at %s, got %#v", sentAt, agent)
	}
}
