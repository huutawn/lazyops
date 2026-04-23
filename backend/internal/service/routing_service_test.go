package service

import (
	"testing"

	"lazyops-server/internal/models"
	"lazyops-server/internal/repository"
)

type fakeServiceRepo struct {
	services []models.Service
}

func newFakeServiceRepo(services []models.Service) *fakeServiceRepo {
	return &fakeServiceRepo{services: services}
}

func (f *fakeServiceRepo) ListByProject(projectID string) ([]models.Service, error) {
	out := make([]models.Service, 0)
	for _, svc := range f.services {
		if svc.ProjectID == projectID {
			out = append(out, svc)
		}
	}
	return out, nil
}

type fakeRoutingPolicyRepo struct {
	policies  map[string]*models.RoutingPolicy
	getErr    error
	upsertErr error
}

func newFakeRoutingPolicyRepo() *fakeRoutingPolicyRepo {
	return &fakeRoutingPolicyRepo{policies: make(map[string]*models.RoutingPolicy)}
}

func (f *fakeRoutingPolicyRepo) GetByProjectID(projectID string) (*models.RoutingPolicy, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.policies[projectID]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (f *fakeRoutingPolicyRepo) Upsert(policy *models.RoutingPolicy) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	copy := *policy
	f.policies[policy.ProjectID] = &copy
	return nil
}

func (f *fakeRoutingPolicyRepo) DeleteByProjectID(projectID string) error {
	delete(f.policies, projectID)
	return nil
}

func TestRoutingServiceGetRoutingEmpty(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo(nil)
	svc := NewRoutingService(repo, svcRepo)

	result, err := svc.GetRouting("usr_123", RoleOperator, "prj_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.RoutingPolicy.Routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(result.RoutingPolicy.Routes))
	}
}

func TestRoutingServiceGetRoutingWithPolicy(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "frontend", Public: true, Kind: "frontend"},
		{ProjectID: "prj_123", Name: "backend", Public: true, Kind: "api"},
	})
	svc := NewRoutingService(repo, svcRepo)

	// Seed a policy
	routesJSON, _ := repository.SerializeRoutes([]models.RoutingRoute{
		{Path: "/", Service: "frontend", Port: 3000},
		{Path: "/api", Service: "backend", Port: 8000},
	})
	repo.policies["prj_123"] = &models.RoutingPolicy{
		ID:           "rp_test",
		ProjectID:    "prj_123",
		SharedDomain: "app.test.sslip.io",
		RoutesJSON:   routesJSON,
	}

	result, err := svc.GetRouting("usr_123", RoleOperator, "prj_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RoutingPolicy.SharedDomain != "app.test.sslip.io" {
		t.Fatalf("expected shared domain 'app.test.sslip.io', got %q", result.RoutingPolicy.SharedDomain)
	}
	if len(result.RoutingPolicy.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(result.RoutingPolicy.Routes))
	}
	if len(result.AvailableServices) != 2 {
		t.Fatalf("expected 2 available services, got %d", len(result.AvailableServices))
	}
	if len(result.SuggestedRoutes) != 2 {
		t.Fatalf("expected suggested routes to be populated, got %#v", result.SuggestedRoutes)
	}
}

func TestRoutingServiceUpdateRoutingSuccess(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "frontend"},
		{ProjectID: "prj_123", Name: "backend"},
	})
	svc := NewRoutingService(repo, svcRepo)

	cmd := UpdateRoutingCommand{
		UserID:       "usr_123",
		Role:         RoleOperator,
		ProjectID:    "prj_123",
		SharedDomain: "app.test.sslip.io",
		Routes: []RoutingRouteRecord{
			{Path: "/", Service: "frontend", Port: 3000},
			{Path: "/api", Service: "backend", Port: 8000, WebSocket: true},
		},
	}

	result, err := svc.UpdateRouting(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RoutingPolicy.SharedDomain != "app.test.sslip.io" {
		t.Fatalf("expected shared domain 'app.test.sslip.io', got %q", result.RoutingPolicy.SharedDomain)
	}
	if len(result.RoutingPolicy.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(result.RoutingPolicy.Routes))
	}
	if !result.RoutingPolicy.Routes[1].WebSocket {
		t.Fatalf("expected second route to have websocket=true")
	}
	// Verify policy was persisted
	policy := repo.policies["prj_123"]
	if policy == nil {
		t.Fatalf("expected routing policy to be persisted")
	}
	if policy.ProjectID != "prj_123" {
		t.Fatalf("expected project ID 'prj_123', got %q", policy.ProjectID)
	}
}

func TestRoutingServiceUpdateRoutingRejectsInvalidService(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "frontend"},
	})
	svc := NewRoutingService(repo, svcRepo)

	cmd := UpdateRoutingCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		ProjectID: "prj_123",
		Routes: []RoutingRouteRecord{
			{Path: "/api", Service: "nonexistent"},
		},
	}

	_, err := svc.UpdateRouting(cmd)
	if err == nil {
		t.Fatalf("expected error for unknown service")
	}
}

func TestRoutingServiceUpdateRoutingRejectsOverlap(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "api"},
	})
	svc := NewRoutingService(repo, svcRepo)

	cmd := UpdateRoutingCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		ProjectID: "prj_123",
		Routes: []RoutingRouteRecord{
			{Path: "/api", Service: "api"},
			{Path: "/api/v1", Service: "api"},
		},
	}

	_, err := svc.UpdateRouting(cmd)
	if err == nil {
		t.Fatalf("expected error for overlapping paths")
	}
}

func TestRoutingServiceUpdateRoutingRejectsEmptyPath(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "api"},
	})
	svc := NewRoutingService(repo, svcRepo)

	cmd := UpdateRoutingCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		ProjectID: "prj_123",
		Routes: []RoutingRouteRecord{
			{Path: "", Service: "api"},
		},
	}

	_, err := svc.UpdateRouting(cmd)
	if err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestRoutingServiceUpdateRoutingRejectsDuplicateRoot(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "frontend"},
		{ProjectID: "prj_123", Name: "backend"},
	})
	svc := NewRoutingService(repo, svcRepo)

	_, err := svc.UpdateRouting(UpdateRoutingCommand{
		UserID:    "usr_123",
		Role:      RoleOperator,
		ProjectID: "prj_123",
		Routes: []RoutingRouteRecord{
			{Path: "/", Service: "frontend"},
			{Path: "/", Service: "backend"},
		},
	})
	if err == nil {
		t.Fatalf("expected duplicate root to fail")
	}
}

func TestRoutingServiceUpdateRoutingRejectsEmptyCmd(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svc := NewRoutingService(repo, nil)

	_, err := svc.UpdateRouting(UpdateRoutingCommand{})
	if err == nil {
		t.Fatalf("expected error for empty command")
	}
}

func TestBuildDefaultRoutingPolicyStripsDefaultAPIPrefix(t *testing.T) {
	policy := buildDefaultRoutingPolicy("", []routingDescriptor{
		{Name: "fe", Kind: "web", Public: true},
		{Name: "be", Kind: "api", Public: true},
	})
	if len(policy.Routes) != 2 {
		t.Fatalf("expected two routes, got %#v", policy.Routes)
	}
	for _, route := range policy.Routes {
		if route.Service == "be" && route.Path == "/api" && !route.StripPrefix {
			t.Fatalf("expected default /api route to strip prefix, got %#v", route)
		}
	}
}

func TestRoutingServiceGetRoutingNormalizesDefaultAPIStripPrefix(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "frontend", Public: true, Kind: "web"},
		{ProjectID: "prj_123", Name: "backend", Public: true, Kind: "api"},
	})
	svc := NewRoutingService(repo, svcRepo)

	routesJSON, _ := repository.SerializeRoutes([]models.RoutingRoute{
		{Path: "/", Service: "frontend", Port: 3000},
		{Path: "/api", Service: "backend", Port: 8000, StripPrefix: false},
	})
	repo.policies["prj_123"] = &models.RoutingPolicy{
		ID:           "rp_test",
		ProjectID:    "prj_123",
		SharedDomain: "app.test.sslip.io",
		RoutesJSON:   routesJSON,
	}

	result, err := svc.GetRouting("usr_123", RoleOperator, "prj_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.RoutingPolicy.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %#v", result.RoutingPolicy.Routes)
	}
	for _, route := range result.RoutingPolicy.Routes {
		if route.Service == "backend" && route.Path == "/api" && !route.StripPrefix {
			t.Fatalf("expected effective /api backend route to strip prefix, got %#v", route)
		}
	}
}

func TestRoutingServiceUpdateRoutingPersistsDefaultAPIStripPrefix(t *testing.T) {
	repo := newFakeRoutingPolicyRepo()
	svcRepo := newFakeServiceRepo([]models.Service{
		{ProjectID: "prj_123", Name: "frontend", Public: true, Kind: "web"},
		{ProjectID: "prj_123", Name: "backend", Public: true, Kind: "api"},
	})
	svc := NewRoutingService(repo, svcRepo)

	result, err := svc.UpdateRouting(UpdateRoutingCommand{
		UserID:       "usr_123",
		Role:         RoleOperator,
		ProjectID:    "prj_123",
		SharedDomain: "app.test.sslip.io",
		Routes: []RoutingRouteRecord{
			{Path: "/", Service: "frontend"},
			{Path: "/api", Service: "backend"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, route := range result.RoutingPolicy.Routes {
		if route.Service == "backend" && route.Path == "/api" && !route.StripPrefix {
			t.Fatalf("expected saved /api backend route to strip prefix, got %#v", route)
		}
	}
	policy := repo.policies["prj_123"]
	if policy == nil {
		t.Fatalf("expected policy persistence")
	}
	routes, err := repository.ParseRoutes(policy.RoutesJSON)
	if err != nil {
		t.Fatalf("parse persisted routes: %v", err)
	}
	for _, route := range routes {
		if route.Service == "backend" && route.Path == "/api" && !route.StripPrefix {
			t.Fatalf("expected persisted /api backend route to strip prefix, got %#v", route)
		}
	}
}
