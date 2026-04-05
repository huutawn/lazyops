package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeTunnelSessionStore struct {
	items     []models.TunnelSession
	createErr error
}

func newFakeTunnelSessionStore(items ...models.TunnelSession) *fakeTunnelSessionStore {
	return &fakeTunnelSessionStore{items: items}
}

func (f *fakeTunnelSessionStore) Create(session *models.TunnelSession) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.items = append(f.items, *session)
	return nil
}

func (f *fakeTunnelSessionStore) GetByID(sessionID string) (*models.TunnelSession, error) {
	for _, item := range f.items {
		if item.ID == sessionID {
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakeTunnelSessionStore) ListByTarget(targetKind, targetID string) ([]models.TunnelSession, error) {
	out := make([]models.TunnelSession, 0)
	for _, item := range f.items {
		if item.TargetKind == targetKind && item.TargetID == targetID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeTunnelSessionStore) CloseSession(sessionID string, at time.Time) error {
	for i, item := range f.items {
		if item.ID == sessionID {
			f.items[i].Status = TunnelSessionStatusClosed
			f.items[i].UpdatedAt = at
			return nil
		}
	}
	return nil
}

func (f *fakeTunnelSessionStore) CleanupExpired(before time.Time) error {
	for i, item := range f.items {
		if item.Status == TunnelSessionStatusActive && item.ExpiresAt.Before(before) {
			f.items[i].Status = TunnelSessionStatusExpired
		}
	}
	return nil
}

type fakeTopologyStateStore struct {
	items []models.TopologyState
}

func newFakeTopologyStateStore(items ...models.TopologyState) *fakeTopologyStateStore {
	return &fakeTopologyStateStore{items: items}
}

func (f *fakeTopologyStateStore) Upsert(state *models.TopologyState) error {
	for i, item := range f.items {
		if item.InstanceID == state.InstanceID {
			f.items[i] = *state
			return nil
		}
	}
	f.items = append(f.items, *state)
	return nil
}

func (f *fakeTopologyStateStore) GetByInstance(instanceID string) (*models.TopologyState, error) {
	for _, item := range f.items {
		if item.InstanceID == instanceID {
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakeTopologyStateStore) ListByProject(projectID string) ([]models.TopologyState, error) {
	return f.items, nil
}

func (f *fakeTopologyStateStore) ListActiveByMesh(meshID string) ([]models.TopologyState, error) {
	out := make([]models.TopologyState, 0)
	for _, item := range f.items {
		if item.MeshID == meshID && item.State != TopologyStateOffline {
			out = append(out, item)
		}
	}
	return out, nil
}

func newTestMeshPlanningService(
	instanceStore InstanceStore,
	bindingStore DeploymentBindingStore,
	revisionStore DesiredStateRevisionStore,
	tunnelStore TunnelSessionStore,
	topologyStore TopologyStateStore,
) *MeshPlanningService {
	return NewMeshPlanningService(instanceStore, bindingStore, revisionStore, tunnelStore, topologyStore)
}

func TestMeshPlanningServiceResolveDependencySuccess(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  ptrString("10.0.1.5"),
		Status:     "online",
		LabelsJSON: `{"services":["api"]}`,
	})

	topologyStore := newFakeTopologyStateStore(models.TopologyState{
		ID:           "topo_123",
		InstanceID:   "inst_123",
		MeshID:       "mesh_123",
		State:        TopologyStateOnline,
		MetadataJSON: `{"region":"sg"}`,
		LastSeenAt:   time.Now().UTC(),
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		TargetRef:  "main",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})

	svc := newTestMeshPlanningService(
		instanceStore,
		bindingStore,
		newFakeDesiredStateRevisionStore(),
		newFakeTunnelSessionStore(),
		topologyStore,
	)

	result, err := svc.ResolveDependencyBinding(context.Background(), "prj_123", "web", LazyopsYAMLDependencyBinding{
		Service:       "web",
		Alias:         "api",
		TargetService: "api",
		Protocol:      "http",
		LocalEndpoint: "localhost:8080",
	})
	if err != nil {
		t.Fatalf("resolve dependency: %v", err)
	}

	if result.TargetService != "api" {
		t.Fatalf("expected target service api, got %q", result.TargetService)
	}
	if result.Protocol != "http" {
		t.Fatalf("expected protocol http, got %q", result.Protocol)
	}
	if !result.PrivatePath.Encrypted {
		t.Fatal("expected private path to be encrypted")
	}
	if result.PrivatePath.Via != "mesh" {
		t.Fatalf("expected private path via mesh, got %q", result.PrivatePath.Via)
	}
	if result.EnvInjection["API_HOST"] != "10.0.1.5" {
		t.Fatalf("expected API_HOST 10.0.1.5, got %q", result.EnvInjection["API_HOST"])
	}
}

func TestMeshPlanningServiceRejectsUnsupportedProtocol(t *testing.T) {
	svc := newTestMeshPlanningService(
		newFakeInstanceStore(),
		newFakeDeploymentBindingStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeTunnelSessionStore(),
		newFakeTopologyStateStore(),
	)

	_, err := svc.ResolveDependencyBinding(context.Background(), "prj_123", "web", LazyopsYAMLDependencyBinding{
		Service:       "web",
		Alias:         "db",
		TargetService: "db",
		Protocol:      "unknown_proto",
		LocalEndpoint: "localhost:5432",
	})
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}

func TestMeshPlanningServiceCreateTunnelSessionSuccess(t *testing.T) {
	privateIP := "10.0.1.5"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &privateIP,
		Status:     "online",
		LabelsJSON: `{}`,
	})

	topologyStore := newFakeTopologyStateStore(models.TopologyState{
		ID:           "topo_123",
		InstanceID:   "inst_123",
		MeshID:       "mesh_123",
		State:        TopologyStateOnline,
		MetadataJSON: `{}`,
		LastSeenAt:   time.Now().UTC(),
	})

	tunnelStore := newFakeTunnelSessionStore()

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		TargetRef:  "main",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})

	svc := newTestMeshPlanningService(
		instanceStore,
		bindingStore,
		newFakeDesiredStateRevisionStore(),
		tunnelStore,
		topologyStore,
	)

	session, err := svc.CreateTunnelSession(context.Background(), "prj_123", "instance", "inst_123", TunnelSessionTypeDB, 5432, 5432, 1*time.Hour)
	if err != nil {
		t.Fatalf("create tunnel session: %v", err)
	}

	if session.ID == "" || session.ID[:4] != "tun_" {
		t.Fatalf("expected tun_ prefixed id, got %q", session.ID)
	}
	if session.Status != TunnelSessionStatusActive {
		t.Fatalf("expected status active, got %q", session.Status)
	}
	if session.SessionType != TunnelSessionTypeDB {
		t.Fatalf("expected session type db, got %q", session.SessionType)
	}
	if session.LocalPort != 5432 {
		t.Fatalf("expected local port 5432, got %d", session.LocalPort)
	}
}

func TestMeshPlanningServiceRejectsOfflineTarget(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_offline",
		UserID:     "usr_123",
		Name:       "edge-sg-2",
		PublicIP:   ptrString("203.0.113.11"),
		PrivateIP:  ptrString("10.0.1.6"),
		Status:     "offline",
		LabelsJSON: `{"services":["db"]}`,
	})

	topologyStore := newFakeTopologyStateStore(models.TopologyState{
		ID:           "topo_offline",
		InstanceID:   "inst_offline",
		MeshID:       "mesh_123",
		State:        TopologyStateOffline,
		MetadataJSON: `{}`,
		LastSeenAt:   time.Now().UTC().Add(-1 * time.Hour),
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		TargetRef:  "main",
		TargetKind: "instance",
		TargetID:   "inst_offline",
	})

	svc := newTestMeshPlanningService(
		instanceStore,
		bindingStore,
		newFakeDesiredStateRevisionStore(),
		newFakeTunnelSessionStore(),
		topologyStore,
	)

	_, err := svc.ResolveDependencyBinding(context.Background(), "prj_123", "api", LazyopsYAMLDependencyBinding{
		Service:       "api",
		Alias:         "db",
		TargetService: "db",
		Protocol:      "tcp",
		LocalEndpoint: "localhost:5432",
	})
	if err == nil {
		t.Fatal("expected error for offline target")
	}
}

func TestMeshPlanningServiceRejectsTunnelToOfflineTarget(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_offline",
		UserID:     "usr_123",
		Name:       "edge-sg-2",
		PublicIP:   ptrString("203.0.113.11"),
		PrivateIP:  ptrString("10.0.1.6"),
		Status:     "offline",
		LabelsJSON: `{}`,
	})

	topologyStore := newFakeTopologyStateStore(models.TopologyState{
		ID:           "topo_offline",
		InstanceID:   "inst_offline",
		MeshID:       "mesh_123",
		State:        TopologyStateOffline,
		MetadataJSON: `{}`,
		LastSeenAt:   time.Now().UTC().Add(-1 * time.Hour),
	})

	svc := newTestMeshPlanningService(
		instanceStore,
		newFakeDeploymentBindingStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeTunnelSessionStore(),
		topologyStore,
	)

	_, err := svc.CreateTunnelSession(context.Background(), "prj_123", "instance", "inst_offline", TunnelSessionTypeDB, 5432, 5432, 1*time.Hour)
	if err == nil {
		t.Fatal("expected error for offline target")
	}
}

func TestMeshPlanningServiceCloseTunnelSession(t *testing.T) {
	tunnelStore := newFakeTunnelSessionStore(models.TunnelSession{
		ID:          "tun_123",
		ProjectID:   "prj_123",
		TargetKind:  "instance",
		TargetID:    "inst_123",
		InstanceID:  "inst_123",
		SessionType: TunnelSessionTypeDB,
		LocalPort:   5432,
		RemotePort:  5432,
		Status:      TunnelSessionStatusActive,
		Token:       "tok_123",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	})

	svc := newTestMeshPlanningService(
		newFakeInstanceStore(),
		newFakeDeploymentBindingStore(),
		newFakeDesiredStateRevisionStore(),
		tunnelStore,
		newFakeTopologyStateStore(),
	)

	result, err := svc.CloseTunnelSession(context.Background(), "tun_123")
	if err != nil {
		t.Fatalf("close tunnel session: %v", err)
	}

	if result.Status != TunnelSessionStatusClosed {
		t.Fatalf("expected status closed, got %q", result.Status)
	}
}

func TestMeshPlanningServiceRejectsTunnelPortConflict(t *testing.T) {
	privateIP := "10.0.1.5"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &privateIP,
		Status:     "online",
		LabelsJSON: `{}`,
	})

	topologyStore := newFakeTopologyStateStore(models.TopologyState{
		ID:           "topo_123",
		InstanceID:   "inst_123",
		MeshID:       "mesh_123",
		State:        TopologyStateOnline,
		MetadataJSON: `{}`,
		LastSeenAt:   time.Now().UTC(),
	})

	tunnelStore := newFakeTunnelSessionStore(models.TunnelSession{
		ID:          "tun_active",
		ProjectID:   "prj_123",
		TargetKind:  "instance",
		TargetID:    "inst_123",
		InstanceID:  "inst_123",
		SessionType: TunnelSessionTypeDB,
		LocalPort:   5432,
		RemotePort:  5432,
		Status:      TunnelSessionStatusActive,
		Token:       "tok_123",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	})

	svc := newTestMeshPlanningService(
		instanceStore,
		newFakeDeploymentBindingStore(),
		newFakeDesiredStateRevisionStore(),
		tunnelStore,
		topologyStore,
	)

	_, err := svc.CreateTunnelSession(context.Background(), "prj_123", "instance", "inst_123", TunnelSessionTypeDB, 5432, 5432, 1*time.Hour)
	if err == nil {
		t.Fatal("expected error for port conflict")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("expected port conflict error, got %v", err)
	}
}

func TestMeshPlanningServiceClosesExpiredSessionBeforeCreatingNew(t *testing.T) {
	privateIP := "10.0.1.5"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &privateIP,
		Status:     "online",
		LabelsJSON: `{}`,
	})

	topologyStore := newFakeTopologyStateStore(models.TopologyState{
		ID:           "topo_123",
		InstanceID:   "inst_123",
		MeshID:       "mesh_123",
		State:        TopologyStateOnline,
		MetadataJSON: `{}`,
		LastSeenAt:   time.Now().UTC(),
	})

	expiredAt := time.Now().Add(-1 * time.Hour)
	tunnelStore := newFakeTunnelSessionStore(models.TunnelSession{
		ID:          "tun_expired",
		ProjectID:   "prj_123",
		TargetKind:  "instance",
		TargetID:    "inst_123",
		InstanceID:  "inst_123",
		SessionType: TunnelSessionTypeDB,
		LocalPort:   5432,
		RemotePort:  5432,
		Status:      TunnelSessionStatusActive,
		Token:       "tok_123",
		ExpiresAt:   expiredAt,
	})

	svc := newTestMeshPlanningService(
		instanceStore,
		newFakeDeploymentBindingStore(),
		newFakeDesiredStateRevisionStore(),
		tunnelStore,
		topologyStore,
	)

	session, err := svc.CreateTunnelSession(context.Background(), "prj_123", "instance", "inst_123", TunnelSessionTypeDB, 5432, 5432, 1*time.Hour)
	if err != nil {
		t.Fatalf("expected expired session to be closed and new session created, got %v", err)
	}
	if session.LocalPort != 5432 {
		t.Fatalf("expected local port 5432, got %d", session.LocalPort)
	}

	closed := false
	for _, s := range tunnelStore.items {
		if s.ID == "tun_expired" && s.Status == TunnelSessionStatusClosed {
			closed = true
		}
	}
	if !closed {
		t.Fatal("expected expired tunnel session to be closed")
	}
}

func TestMeshPlanningServiceIngestTopologyState(t *testing.T) {
	topologyStore := newFakeTopologyStateStore()

	svc := newTestMeshPlanningService(
		newFakeInstanceStore(),
		newFakeDeploymentBindingStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeTunnelSessionStore(),
		topologyStore,
	)

	result, err := svc.IngestTopologyState(context.Background(), "inst_123", "mesh_123", "online", map[string]any{"cpu": 45.2})
	if err != nil {
		t.Fatalf("ingest topology state: %v", err)
	}

	if result.InstanceID != "inst_123" {
		t.Fatalf("expected instance id inst_123, got %q", result.InstanceID)
	}
	if result.State != TopologyStateOnline {
		t.Fatalf("expected state online, got %q", result.State)
	}
	if result.Metadata["cpu"] != 45.2 {
		t.Fatalf("expected cpu 45.2, got %v", result.Metadata["cpu"])
	}
}

func TestMeshPlanningServiceListMeshTopology(t *testing.T) {
	topologyStore := newFakeTopologyStateStore(
		models.TopologyState{
			ID:           "topo_1",
			InstanceID:   "inst_1",
			MeshID:       "mesh_123",
			State:        TopologyStateOnline,
			MetadataJSON: `{"cpu":45}`,
			LastSeenAt:   time.Now().UTC(),
		},
		models.TopologyState{
			ID:           "topo_2",
			InstanceID:   "inst_2",
			MeshID:       "mesh_123",
			State:        TopologyStateDegraded,
			MetadataJSON: `{"cpu":90}`,
			LastSeenAt:   time.Now().UTC(),
		},
	)

	svc := newTestMeshPlanningService(
		newFakeInstanceStore(),
		newFakeDeploymentBindingStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeTunnelSessionStore(),
		topologyStore,
	)

	states, err := svc.ListMeshTopology("prj_123")
	if err != nil {
		t.Fatalf("list mesh topology: %v", err)
	}

	if len(states) != 2 {
		t.Fatalf("expected 2 topology states, got %d", len(states))
	}
}

func TestNormalizeTopologyState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"online", TopologyStateOnline},
		{"offline", TopologyStateOffline},
		{"degraded", TopologyStateDegraded},
		{"unknown", TopologyStateOffline},
		{"", TopologyStateOffline},
	}

	for _, tt := range tests {
		got := normalizeTopologyState(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeTopologyState(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractPortFromDependencyBinding(t *testing.T) {
	tests := []struct {
		binding  LazyopsYAMLDependencyBinding
		expected int
	}{
		{LazyopsYAMLDependencyBinding{LocalEndpoint: "localhost:8080"}, 8080},
		{LazyopsYAMLDependencyBinding{LocalEndpoint: "localhost:5432"}, 5432},
		{LazyopsYAMLDependencyBinding{LocalEndpoint: ""}, 8080},
	}

	for _, tt := range tests {
		got := extractPortFromDependencyBinding(tt.binding)
		if got != tt.expected {
			t.Errorf("extractPortFromDependencyBinding(%v) = %d, want %d", tt.binding, got, tt.expected)
		}
	}
}
