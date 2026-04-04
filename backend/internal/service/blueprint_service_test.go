package service

import (
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeProjectServiceStore struct {
	byProjectID map[string][]models.Service
	replaceErr  error
	listErr     error
}

func newFakeProjectServiceStore() *fakeProjectServiceStore {
	return &fakeProjectServiceStore{
		byProjectID: make(map[string][]models.Service),
	}
}

func (f *fakeProjectServiceStore) ReplaceForProject(projectID string, items []models.Service) error {
	if f.replaceErr != nil {
		return f.replaceErr
	}

	cloned := make([]models.Service, 0, len(items))
	now := time.Now().UTC()
	for _, item := range items {
		copyItem := item
		if copyItem.CreatedAt.IsZero() {
			copyItem.CreatedAt = now
		}
		if copyItem.UpdatedAt.IsZero() {
			copyItem.UpdatedAt = copyItem.CreatedAt
		}
		cloned = append(cloned, copyItem)
	}
	f.byProjectID[projectID] = cloned
	return nil
}

func (f *fakeProjectServiceStore) ListByProject(projectID string) ([]models.Service, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	items := f.byProjectID[projectID]
	out := make([]models.Service, 0, len(items))
	out = append(out, items...)
	return out, nil
}

type fakeBlueprintStore struct {
	items     []models.Blueprint
	createErr error
	getErr    error
}

func newFakeBlueprintStore() *fakeBlueprintStore {
	return &fakeBlueprintStore{}
}

func (f *fakeBlueprintStore) Create(blueprint *models.Blueprint) error {
	if f.createErr != nil {
		return f.createErr
	}
	copyItem := *blueprint
	if copyItem.CreatedAt.IsZero() {
		copyItem.CreatedAt = time.Now().UTC()
	}
	blueprint.CreatedAt = copyItem.CreatedAt
	f.items = append(f.items, copyItem)
	return nil
}

func (f *fakeBlueprintStore) GetByIDForProject(projectID, blueprintID string) (*models.Blueprint, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	for _, item := range f.items {
		if item.ProjectID == projectID && item.ID == blueprintID {
			copyItem := item
			return &copyItem, nil
		}
	}

	return nil, nil
}

func TestBlueprintServiceCompileSuccess(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "acme-api",
		TrackedBranch:        "main",
		PreviewEnabled:       true,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Production Binding",
		TargetRef:               "prod-main",
		RuntimeMode:             "standalone",
		TargetKind:              "instance",
		TargetID:                "inst_123",
		PlacementPolicyJSON:     `{"strategy":"single-node","labels":{"region":"sg"}}`,
		DomainPolicyJSON:        `{"magic_domain_provider":"sslip.io"}`,
		CompatibilityPolicyJSON: `{"env_injection":true,"managed_credentials":true,"localhost_rescue":true}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	serviceStore := newFakeProjectServiceStore()
	blueprintStore := newFakeBlueprintStore()
	service := NewBlueprintService(projectStore, repoLinkStore, bindingStore, serviceStore, blueprintStore)

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"standalone",
		"deployment_binding":{"target_ref":"prod-main"},
		"services":[
			{"name":"web","path":"apps/web","public":true,"healthcheck":{"path":"/health","port":3000}},
			{"name":"api","path":"apps/api","start_hint":"go run ./cmd/server","healthcheck":{"path":"/healthz","port":8080}}
		],
		"dependency_bindings":[
			{"service":"web","alias":"api","target_service":"api","protocol":"http","local_endpoint":"localhost:8080"}
		],
		"compatibility_policy":{"env_injection":true,"managed_credentials":true,"localhost_rescue":true},
		"magic_domain_policy":{"enabled":true,"provider":"sslip.io"},
		"scale_to_zero_policy":{"enabled":false}
	}`)

	result, err := service.Compile(CompileBlueprintCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Artifact: BlueprintArtifactMetadata{
			CommitSHA:   "abc123def456",
			ArtifactRef: "artifact://builds/123",
			ImageRef:    "ghcr.io/lazyops/acme-api:abc123",
		},
		LazyopsYAMLRaw: raw,
	})
	if err != nil {
		t.Fatalf("compile blueprint: %v", err)
	}

	if result.Blueprint.ID == "" || result.Blueprint.ID[:3] != "bp_" {
		t.Fatalf("expected blueprint id with bp_ prefix, got %q", result.Blueprint.ID)
	}
	if result.Blueprint.SourceKind != "lazyops_yaml" {
		t.Fatalf("expected source kind lazyops_yaml, got %q", result.Blueprint.SourceKind)
	}
	if result.Blueprint.Compiled.Repo.RepoFullName != "lazyops/acme-api" {
		t.Fatalf("expected repo full name lazyops/acme-api, got %q", result.Blueprint.Compiled.Repo.RepoFullName)
	}
	if len(result.Services) != 2 {
		t.Fatalf("expected 2 persisted service records, got %d", len(result.Services))
	}
	if result.Services[0].RuntimeProfile == "" || result.Services[1].RuntimeProfile == "" {
		t.Fatalf("expected runtime profiles to be inferred, got %#v", result.Services)
	}
	if len(result.DesiredRevisionDraft.Services) != 2 {
		t.Fatalf("expected 2 draft services, got %d", len(result.DesiredRevisionDraft.Services))
	}
	if len(result.DesiredRevisionDraft.PlacementAssignments) != 2 {
		t.Fatalf("expected 2 placement assignments, got %d", len(result.DesiredRevisionDraft.PlacementAssignments))
	}
	persisted, err := serviceStore.ListByProject("prj_123")
	if err != nil {
		t.Fatalf("list persisted services: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("expected 2 persisted services in store, got %d", len(persisted))
	}
	if len(blueprintStore.items) != 1 {
		t.Fatalf("expected 1 blueprint record, got %d", len(blueprintStore.items))
	}
}

func TestBlueprintServiceRejectsInvalidLazyopsYAML(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "acme-api",
		TrackedBranch:        "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production Binding",
		TargetRef:   "prod-main",
		RuntimeMode: "standalone",
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	service := NewBlueprintService(projectStore, repoLinkStore, bindingStore, newFakeProjectServiceStore(), newFakeBlueprintStore())

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"standalone",
		"deployment_binding":{"target_ref":"prod-main"},
		"services":[],
		"compatibility_policy":{"env_injection":true}
	}`)

	_, err := service.Compile(CompileBlueprintCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Artifact: BlueprintArtifactMetadata{
			CommitSHA: "abc123def456",
		},
		LazyopsYAMLRaw: raw,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestBlueprintServiceRejectsMissingBinding(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "acme-api",
		TrackedBranch:        "main",
	})
	service := NewBlueprintService(projectStore, repoLinkStore, newFakeDeploymentBindingStore(), newFakeProjectServiceStore(), newFakeBlueprintStore())

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"standalone",
		"deployment_binding":{"target_ref":"prod-main"},
		"services":[{"name":"api","path":"apps/api"}],
		"compatibility_policy":{"env_injection":true}
	}`)

	_, err := service.Compile(CompileBlueprintCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Artifact: BlueprintArtifactMetadata{
			CommitSHA: "abc123def456",
		},
		LazyopsYAMLRaw: raw,
	})
	if !errors.Is(err, ErrUnknownTargetRef) {
		t.Fatalf("expected ErrUnknownTargetRef, got %v", err)
	}
}
