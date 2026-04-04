package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakePublicRouteStore struct {
	items     []models.PublicRoute
	createErr error
}

func newFakePublicRouteStore(items ...models.PublicRoute) *fakePublicRouteStore {
	return &fakePublicRouteStore{items: items}
}

func (f *fakePublicRouteStore) Create(route *models.PublicRoute) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.items = append(f.items, *route)
	return nil
}

func (f *fakePublicRouteStore) ListByProject(projectID string) ([]models.PublicRoute, error) {
	out := make([]models.PublicRoute, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakePublicRouteStore) ListByDeployment(projectID, deploymentID string) ([]models.PublicRoute, error) {
	out := make([]models.PublicRoute, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID && item.DeploymentID == deploymentID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakePublicRouteStore) UpdateStatus(routeID, status string, at time.Time) error {
	for i, item := range f.items {
		if item.ID == routeID {
			f.items[i].Status = status
			f.items[i].UpdatedAt = at
			return nil
		}
	}
	return nil
}

type fakeGatewayConfigIntentStore struct {
	items     []models.GatewayConfigIntent
	createErr error
}

func newFakeGatewayConfigIntentStore(items ...models.GatewayConfigIntent) *fakeGatewayConfigIntentStore {
	return &fakeGatewayConfigIntentStore{items: items}
}

func (f *fakeGatewayConfigIntentStore) Create(intent *models.GatewayConfigIntent) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.items = append(f.items, *intent)
	return nil
}

func (f *fakeGatewayConfigIntentStore) GetByIDForProject(projectID, intentID string) (*models.GatewayConfigIntent, error) {
	for _, item := range f.items {
		if item.ProjectID == projectID && item.ID == intentID {
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakeGatewayConfigIntentStore) ListByDeployment(projectID, deploymentID string) ([]models.GatewayConfigIntent, error) {
	out := make([]models.GatewayConfigIntent, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID && item.DeploymentID == deploymentID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeGatewayConfigIntentStore) UpdateStatus(intentID, status string, at time.Time) error {
	for i, item := range f.items {
		if item.ID == intentID {
			f.items[i].Status = status
			f.items[i].UpdatedAt = at
			return nil
		}
	}
	return nil
}

type fakeReleaseHistoryStore struct {
	items     []models.ReleaseHistory
	createErr error
}

func newFakeReleaseHistoryStore(items ...models.ReleaseHistory) *fakeReleaseHistoryStore {
	return &fakeReleaseHistoryStore{items: items}
}

func (f *fakeReleaseHistoryStore) Create(record *models.ReleaseHistory) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.items = append(f.items, *record)
	return nil
}

func (f *fakeReleaseHistoryStore) ListByProject(projectID string, limit int) ([]models.ReleaseHistory, error) {
	out := make([]models.ReleaseHistory, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeReleaseHistoryStore) GetByIDForProject(projectID, recordID string) (*models.ReleaseHistory, error) {
	for _, item := range f.items {
		if item.ProjectID == projectID && item.ID == recordID {
			return &item, nil
		}
	}
	return nil, nil
}

func newTestGatewayConfigService(
	revisionStore DesiredStateRevisionStore,
	deploymentStore DeploymentStore,
	bindingStore DeploymentBindingStore,
	routeStore PublicRouteStore,
	intentStore GatewayConfigIntentStore,
	releaseStore ReleaseHistoryStore,
) *GatewayConfigService {
	return NewGatewayConfigService(revisionStore, deploymentStore, bindingStore, routeStore, intentStore, releaseStore)
}

func TestGatewayConfigServiceAllocateMagicDomainSuccessSSLIP(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	domain, err := svc.AllocateMagicDomain("prj_123", "api", "203.0.113.10", "sslip.io")
	if err != nil {
		t.Fatalf("allocate magic domain: %v", err)
	}

	expected := "api.203-0-113-10.sslip.io"
	if domain != expected {
		t.Fatalf("expected domain %q, got %q", expected, domain)
	}
}

func TestGatewayConfigServiceAllocateMagicDomainDefaultSSLIP(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	domain, err := svc.AllocateMagicDomain("prj_123", "api", "203.0.113.10", "")
	if err != nil {
		t.Fatalf("allocate magic domain: %v", err)
	}

	expected := "api.203-0-113-10.sslip.io"
	if domain != expected {
		t.Fatalf("expected domain %q, got %q", expected, domain)
	}
}

func TestGatewayConfigServiceAllocateMagicDomainFallbackNipIO(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	domain, err := svc.AllocateMagicDomain("prj_123", "api", "203.0.113.10", "nip.io")
	if err != nil {
		t.Fatalf("allocate magic domain: %v", err)
	}

	expected := "api.203-0-113-10.nip.io"
	if domain != expected {
		t.Fatalf("expected domain %q, got %q", expected, domain)
	}
}

func TestGatewayConfigServiceRejectsPrivateIPForMagicDomain(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	privateIPs := []string{"192.168.1.1", "10.0.0.1", "172.16.0.1", "127.0.0.1"}
	for _, ip := range privateIPs {
		_, err := svc.AllocateMagicDomain("prj_123", "api", ip, "")
		if err == nil {
			t.Fatalf("expected error for private IP %q, got nil", ip)
		}
	}
}

func TestGatewayConfigServiceRejectsInvalidIPForMagicDomain(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	_, err := svc.AllocateMagicDomain("prj_123", "api", "not-an-ip", "")
	if err == nil {
		t.Fatal("expected error for invalid IP")
	}

	_, err = svc.AllocateMagicDomain("prj_123", "api", "", "")
	if err == nil {
		t.Fatal("expected error for empty IP")
	}
}

func TestGatewayConfigServiceGenerateGatewayConfigSuccess(t *testing.T) {
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: "standalone",
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})

	routeStore := newFakePublicRouteStore(models.PublicRoute{
		ID:           "prt_123",
		ProjectID:    "prj_123",
		DeploymentID: "dep_123",
		ServiceName:  "api",
		Domain:       "api.203-0-113-10.sslip.io",
		DomainKind:   DomainKindMagic,
		PathPrefix:   "/",
		UpstreamPort: 8080,
		HTTPS:        true,
		Status:       PublicRouteStatusActive,
	})

	svc := newTestGatewayConfigService(
		revisionStore,
		newFakeDeploymentStore(),
		bindingStore,
		routeStore,
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	config, err := svc.GenerateGatewayConfig(context.Background(), "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("generate gateway config: %v", err)
	}

	if config.RuntimeMode != "standalone" {
		t.Fatalf("expected runtime mode standalone, got %q", config.RuntimeMode)
	}
	if config.TargetKind != "instance" {
		t.Fatalf("expected target kind instance, got %q", config.TargetKind)
	}
	if config.TargetID != "inst_123" {
		t.Fatalf("expected target id inst_123, got %q", config.TargetID)
	}
	if len(config.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(config.Routes))
	}
	if config.Routes[0].Domain != "api.203-0-113-10.sslip.io" {
		t.Fatalf("expected route domain api.203-0-113-10.sslip.io, got %q", config.Routes[0].Domain)
	}
}

func TestGatewayConfigServiceDispatchGatewayConfigSuccess(t *testing.T) {
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: "standalone",
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})

	intentStore := newFakeGatewayConfigIntentStore()

	svc := newTestGatewayConfigService(
		revisionStore,
		newFakeDeploymentStore(),
		bindingStore,
		newFakePublicRouteStore(),
		intentStore,
		newFakeReleaseHistoryStore(),
	)

	result, err := svc.DispatchGatewayConfig(context.Background(), "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("dispatch gateway config: %v", err)
	}

	if result.IntentID == "" || result.IntentID[:4] != "gwi_" {
		t.Fatalf("expected gwi_ prefixed id, got %q", result.IntentID)
	}
	if result.Status != GatewayConfigStatusDispatched {
		t.Fatalf("expected status dispatched, got %q", result.Status)
	}

	if len(intentStore.items) != 1 {
		t.Fatalf("expected 1 intent created, got %d", len(intentStore.items))
	}
}

func TestGatewayConfigServiceRecordReleaseSuccess(t *testing.T) {
	releaseStore := newFakeReleaseHistoryStore()

	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		releaseStore,
	)

	record, err := svc.RecordRelease("prj_123", "dep_123", "rev_123", "abc123", "push", "standalone", ReleaseStatusDeployed)
	if err != nil {
		t.Fatalf("record release: %v", err)
	}

	if record.ID == "" || record.ID[:4] != "rel_" {
		t.Fatalf("expected rel_ prefixed id, got %q", record.ID)
	}
	if record.CommitSHA != "abc123" {
		t.Fatalf("expected commit sha abc123, got %q", record.CommitSHA)
	}
	if record.Status != ReleaseStatusDeployed {
		t.Fatalf("expected status deployed, got %q", record.Status)
	}
	if record.DeployedAt == nil {
		t.Fatal("expected deployed_at to be set for deployed release")
	}
}

func TestGatewayConfigServiceListReleaseHistory(t *testing.T) {
	releaseStore := newFakeReleaseHistoryStore(
		models.ReleaseHistory{
			ID:           "rel_1",
			ProjectID:    "prj_123",
			DeploymentID: "dep_1",
			RevisionID:   "rev_1",
			CommitSHA:    "abc123",
			TriggerKind:  "push",
			Status:       ReleaseStatusDeployed,
			RuntimeMode:  "standalone",
			SummaryJSON:  `{"commit_sha":"abc123"}`,
			CreatedAt:    time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC),
		},
		models.ReleaseHistory{
			ID:           "rel_2",
			ProjectID:    "prj_123",
			DeploymentID: "dep_2",
			RevisionID:   "rev_2",
			CommitSHA:    "def456",
			TriggerKind:  "manual",
			Status:       ReleaseStatusFailed,
			RuntimeMode:  "standalone",
			SummaryJSON:  `{"commit_sha":"def456"}`,
			CreatedAt:    time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC),
		},
	)

	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		releaseStore,
	)

	history, err := svc.ListReleaseHistory("prj_123", 10)
	if err != nil {
		t.Fatalf("list release history: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(history))
	}
}

func TestGatewayConfigServiceGetReleaseDetail(t *testing.T) {
	releaseStore := newFakeReleaseHistoryStore(
		models.ReleaseHistory{
			ID:           "rel_123",
			ProjectID:    "prj_123",
			DeploymentID: "dep_123",
			RevisionID:   "rev_123",
			CommitSHA:    "abc123",
			TriggerKind:  "push",
			Status:       ReleaseStatusDeployed,
			RuntimeMode:  "standalone",
			SummaryJSON:  `{"commit_sha":"abc123","trigger_kind":"push"}`,
			CreatedAt:    time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC),
		},
	)

	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		releaseStore,
	)

	detail, err := svc.GetReleaseDetail("prj_123", "rel_123")
	if err != nil {
		t.Fatalf("get release detail: %v", err)
	}

	if detail.ID != "rel_123" {
		t.Fatalf("expected id rel_123, got %q", detail.ID)
	}
	if detail.CommitSHA != "abc123" {
		t.Fatalf("expected commit sha abc123, got %q", detail.CommitSHA)
	}
	if detail.Summary["commit_sha"] != "abc123" {
		t.Fatalf("expected summary commit_sha abc123, got %v", detail.Summary["commit_sha"])
	}
}

func TestGatewayConfigServiceGetReleaseDetailNotFound(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	_, err := svc.GetReleaseDetail("prj_123", "rel_missing")
	if err == nil {
		t.Fatal("expected error for missing release")
	}
}

func TestGatewayConfigServiceCreatePublicRoute(t *testing.T) {
	routeStore := newFakePublicRouteStore()

	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		routeStore,
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	route, err := svc.CreatePublicRoute("prj_123", "dep_123", "api", "api.203-0-113-10.sslip.io", DomainKindMagic, "/", 8080, true)
	if err != nil {
		t.Fatalf("create public route: %v", err)
	}

	if route.ID == "" || route.ID[:4] != "prt_" {
		t.Fatalf("expected prt_ prefixed id, got %q", route.ID)
	}
	if route.Domain != "api.203-0-113-10.sslip.io" {
		t.Fatalf("expected domain api.203-0-113-10.sslip.io, got %q", route.Domain)
	}
	if route.DomainKind != DomainKindMagic {
		t.Fatalf("expected domain kind magic, got %q", route.DomainKind)
	}
	if route.UpstreamPort != 8080 {
		t.Fatalf("expected upstream port 8080, got %d", route.UpstreamPort)
	}
	if !route.HTTPS {
		t.Fatal("expected HTTPS to be true")
	}
}

func TestGatewayConfigServiceRejectsEmptyDomain(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
	)

	_, err := svc.CreatePublicRoute("prj_123", "dep_123", "api", "", DomainKindMagic, "/", 8080, true)
	if err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestNormalizePathPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{"api", "/api"},
		{"/api", "/api"},
		{"  /health  ", "/health"},
	}

	for _, tt := range tests {
		got := normalizePathPrefix(tt.input)
		if got != tt.expected {
			t.Errorf("normalizePathPrefix(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsPrivateIP(t *testing.T) {
	publicIPs := []string{"203.0.113.10", "8.8.8.8", "1.1.1.1"}
	for _, ip := range publicIPs {
		if isPrivateIP(ip) {
			t.Errorf("expected %q to be public", ip)
		}
	}

	privateIPs := []string{"192.168.1.1", "10.0.0.1", "172.16.0.1", "127.0.0.1"}
	for _, ip := range privateIPs {
		if !isPrivateIP(ip) {
			t.Errorf("expected %q to be private", ip)
		}
	}
}

func TestExtractPortFromHealthcheck(t *testing.T) {
	tests := []struct {
		hc       map[string]any
		expected int
	}{
		{nil, 8080},
		{map[string]any{"port": 3000}, 3000},
		{map[string]any{"port": float64(3000)}, 3000},
		{map[string]any{"path": "/health"}, 8080},
	}

	for _, tt := range tests {
		got := extractPortFromHealthcheck(tt.hc)
		if got != tt.expected {
			t.Errorf("extractPortFromHealthcheck(%v) = %d, want %d", tt.hc, got, tt.expected)
		}
	}
}

func TestGatewayConfigPayloadMarshals(t *testing.T) {
	payload := GatewayConfigPayload{
		ProjectID:    "prj_123",
		DeploymentID: "dep_123",
		RevisionID:   "rev_123",
		RuntimeMode:  "standalone",
		TargetKind:   "instance",
		TargetID:     "inst_123",
		Routes: []RouteConfig{
			{
				Domain:       "api.203-0-113-10.sslip.io",
				ServiceName:  "api",
				PathPrefix:   "/",
				UpstreamPort: 8080,
				HTTPS:        true,
			},
		},
		CompatibilityPolicy: LazyopsYAMLCompatibilityPolicy{
			EnvInjection:       true,
			ManagedCredentials: true,
			LocalhostRescue:    true,
		},
		ScaleToZeroPolicy: LazyopsYAMLScaleToZeroPolicy{
			Enabled: false,
		},
		WakeUpHold: WakeUpHoldConfig{
			Enabled:      true,
			TimeoutMs:    5000,
			HoldResponse: "warming up",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var decoded GatewayConfigPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if decoded.ProjectID != "prj_123" {
		t.Fatalf("expected project id prj_123, got %q", decoded.ProjectID)
	}
	if len(decoded.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(decoded.Routes))
	}
	if !decoded.WakeUpHold.Enabled {
		t.Fatal("expected wake up hold enabled")
	}
}
