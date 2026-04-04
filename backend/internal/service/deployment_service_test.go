package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeDesiredStateRevisionStore struct {
	byProjectID map[string]map[string]*models.DesiredStateRevision
	createErr   error
	getErr      error
	updateErr   error
}

func newFakeDesiredStateRevisionStore(items ...*models.DesiredStateRevision) *fakeDesiredStateRevisionStore {
	store := &fakeDesiredStateRevisionStore{
		byProjectID: make(map[string]map[string]*models.DesiredStateRevision),
	}
	for _, item := range items {
		store.put(item)
	}
	return store
}

func (f *fakeDesiredStateRevisionStore) Create(revision *models.DesiredStateRevision) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *revision
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	revision.CreatedAt = cloned.CreatedAt
	revision.UpdatedAt = cloned.UpdatedAt
	f.put(&cloned)
	return nil
}

func (f *fakeDesiredStateRevisionStore) GetByIDForProject(projectID, revisionID string) (*models.DesiredStateRevision, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	projectItems := f.byProjectID[projectID]
	if projectItems == nil {
		return nil, nil
	}
	if item, ok := projectItems[revisionID]; ok {
		return item, nil
	}
	return nil, nil
}

func (f *fakeDesiredStateRevisionStore) UpdateStatus(revisionID, status string, at time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	for _, projectItems := range f.byProjectID {
		if item, ok := projectItems[revisionID]; ok {
			item.Status = status
			item.UpdatedAt = at
			return nil
		}
	}
	return nil
}

func (f *fakeDesiredStateRevisionStore) ListByProject(projectID string) ([]models.DesiredStateRevision, error) {
	projectItems := f.byProjectID[projectID]
	if projectItems == nil {
		return nil, nil
	}
	out := make([]models.DesiredStateRevision, 0, len(projectItems))
	for _, item := range projectItems {
		out = append(out, *item)
	}
	return out, nil
}

func (f *fakeDesiredStateRevisionStore) put(item *models.DesiredStateRevision) {
	projectItems := f.byProjectID[item.ProjectID]
	if projectItems == nil {
		projectItems = make(map[string]*models.DesiredStateRevision)
		f.byProjectID[item.ProjectID] = projectItems
	}
	cloned := *item
	projectItems[item.ID] = &cloned
}

type fakeDeploymentStore struct {
	byProjectID map[string]map[string]*models.Deployment
	createErr   error
	getErr      error
	updateErr   error
}

func newFakeDeploymentStore(items ...*models.Deployment) *fakeDeploymentStore {
	store := &fakeDeploymentStore{
		byProjectID: make(map[string]map[string]*models.Deployment),
	}
	for _, item := range items {
		store.put(item)
	}
	return store
}

func (f *fakeDeploymentStore) Create(deployment *models.Deployment) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *deployment
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	deployment.CreatedAt = cloned.CreatedAt
	deployment.UpdatedAt = cloned.UpdatedAt
	f.put(&cloned)
	return nil
}

func (f *fakeDeploymentStore) GetByIDForProject(projectID, deploymentID string) (*models.Deployment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	projectItems := f.byProjectID[projectID]
	if projectItems == nil {
		return nil, nil
	}
	if item, ok := projectItems[deploymentID]; ok {
		return item, nil
	}
	return nil, nil
}

func (f *fakeDeploymentStore) UpdateStatus(deploymentID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	for _, projectItems := range f.byProjectID {
		if item, ok := projectItems[deploymentID]; ok {
			item.Status = status
			item.UpdatedAt = updatedAt
			if startedAt != nil {
				item.StartedAt = startedAt
			}
			if completedAt != nil {
				item.CompletedAt = completedAt
			}
			return nil
		}
	}
	return nil
}

func (f *fakeDeploymentStore) put(item *models.Deployment) {
	projectItems := f.byProjectID[item.ProjectID]
	if projectItems == nil {
		projectItems = make(map[string]*models.Deployment)
		f.byProjectID[item.ProjectID] = projectItems
	}
	cloned := *item
	projectItems[item.ID] = &cloned
}

func TestDeploymentServiceCreateSuccess(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	service := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore)

	result, err := service.Create(CreateDeploymentCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		BlueprintID:     "bp_123",
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	if result.Revision.ID == "" || result.Revision.ID[:4] != "rev_" {
		t.Fatalf("expected rev_ prefixed id, got %q", result.Revision.ID)
	}
	if result.Revision.Status != RevisionStatusQueued {
		t.Fatalf("expected revision status queued, got %q", result.Revision.Status)
	}
	if result.Revision.DeploymentBindingID != "bind_123" {
		t.Fatalf("expected deployment binding bind_123, got %q", result.Revision.DeploymentBindingID)
	}
	if result.Deployment.ID == "" || result.Deployment.ID[:4] != "dep_" {
		t.Fatalf("expected dep_ prefixed id, got %q", result.Deployment.ID)
	}
	if result.Deployment.RevisionID != result.Revision.ID {
		t.Fatalf("expected deployment to point at revision %q, got %q", result.Revision.ID, result.Deployment.RevisionID)
	}
	if result.Deployment.Status != DeploymentStatusQueued {
		t.Fatalf("expected deployment status queued, got %q", result.Deployment.Status)
	}
	if len(result.Revision.Services) != 2 {
		t.Fatalf("expected 2 services in revision, got %d", len(result.Revision.Services))
	}
}

func TestDeploymentServiceRejectsOwnershipMismatch(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_owner",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))
	service := NewDeploymentService(projectStore, blueprintStore, newFakeDesiredStateRevisionStore(), newFakeDeploymentStore())

	_, err := service.Create(CreateDeploymentCommand{
		RequesterUserID: "usr_other",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		BlueprintID:     "bp_123",
	})
	if !errors.Is(err, ErrProjectAccessDenied) {
		t.Fatalf("expected ErrProjectAccessDenied, got %v", err)
	}
}

func TestDeploymentServiceRejectsInvalidRevisionTransition(t *testing.T) {
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123def456",
		TriggerKind:          "manual",
		Status:               RevisionStatusPromoted,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
		CreatedAt:            time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC),
	})
	service := NewDeploymentService(newFakeProjectStore(), newFakeBlueprintStore(), revisionStore, newFakeDeploymentStore())

	_, err := service.TransitionRevisionStatus("prj_123", "rev_123", RevisionStatusBuilding)
	if !errors.Is(err, ErrInvalidRevisionStateTransition) {
		t.Fatalf("expected ErrInvalidRevisionStateTransition, got %v", err)
	}
}

func TestDeploymentServicePersistsDeploymentRecord(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	service := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore)

	result, err := service.Create(CreateDeploymentCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		BlueprintID:     "bp_123",
		TriggerKind:     "manual_promote",
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	projectDeployments := deploymentStore.byProjectID["prj_123"]
	if len(projectDeployments) != 1 {
		t.Fatalf("expected 1 deployment in store, got %d", len(projectDeployments))
	}
	stored := projectDeployments[result.Deployment.ID]
	if stored == nil {
		t.Fatalf("expected deployment %q to be persisted", result.Deployment.ID)
	}
	if stored.RevisionID != result.Revision.ID {
		t.Fatalf("expected stored deployment revision id %q, got %q", result.Revision.ID, stored.RevisionID)
	}
	if stored.StartedAt != nil || stored.CompletedAt != nil {
		t.Fatalf("expected queued deployment to have nil lifecycle timestamps, got %#v", stored)
	}
}

func mustBlueprintModel(t *testing.T, blueprintID, projectID string) models.Blueprint {
	t.Helper()

	compiledJSON, err := json.Marshal(BlueprintCompiledContractRecord{
		ProjectID:   projectID,
		RuntimeMode: "standalone",
		Repo: BlueprintRepoStateRecord{
			ProjectRepoLinkID: "prl_123",
			RepoOwner:         "lazyops",
			RepoName:          "acme-api",
			RepoFullName:      "lazyops/acme-api",
			TrackedBranch:     "main",
			PreviewEnabled:    true,
		},
		Binding: DeploymentBindingRecord{
			ID:                  "bind_123",
			ProjectID:           projectID,
			Name:                "Production",
			TargetRef:           "prod-main",
			RuntimeMode:         "standalone",
			TargetKind:          "instance",
			TargetID:            "inst_123",
			PlacementPolicy:     map[string]any{"labels": map[string]any{"region": "sg"}},
			DomainPolicy:        map[string]any{"magic_domain_provider": "sslip.io"},
			CompatibilityPolicy: map[string]any{"env_injection": true, "managed_credentials": true, "localhost_rescue": true},
			ScaleToZeroPolicy:   map[string]any{"enabled": false},
		},
		Services: []BlueprintServiceContractRecord{
			{
				Name:           "web",
				Path:           "apps/web",
				Public:         true,
				RuntimeProfile: "web",
				Healthcheck:    map[string]any{"path": "/health", "port": 3000, "protocol": "http"},
			},
			{
				Name:           "api",
				Path:           "apps/api",
				RuntimeProfile: "service",
				StartHint:      "go run ./cmd/server",
				Healthcheck:    map[string]any{"path": "/healthz", "port": 8080, "protocol": "http"},
			},
		},
		DependencyBindings: []LazyopsYAMLDependencyBinding{
			{
				Service:       "web",
				Alias:         "api",
				TargetService: "api",
				Protocol:      "http",
				LocalEndpoint: "localhost:8080",
			},
		},
		CompatibilityPolicy: LazyopsYAMLCompatibilityPolicy{
			EnvInjection:       true,
			ManagedCredentials: true,
			LocalhostRescue:    true,
		},
		MagicDomainPolicy: LazyopsYAMLMagicDomainPolicy{
			Enabled:  true,
			Provider: "sslip.io",
		},
		ScaleToZeroPolicy: LazyopsYAMLScaleToZeroPolicy{
			Enabled: false,
		},
		ArtifactMetadata: BlueprintArtifactMetadata{
			CommitSHA:   "abc123def456",
			ArtifactRef: "artifact://builds/123",
			ImageRef:    "ghcr.io/lazyops/acme-api:abc123",
		},
	})
	if err != nil {
		t.Fatalf("marshal blueprint compiled json: %v", err)
	}

	return models.Blueprint{
		ID:           blueprintID,
		ProjectID:    projectID,
		SourceKind:   "lazyops_yaml",
		SourceRef:    "lazyops/acme-api@main",
		CompiledJSON: string(compiledJSON),
		CreatedAt:    time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC),
	}
}

func mustCompiledRevisionJSON(t *testing.T, revisionID, blueprintID, projectID string) string {
	t.Helper()

	raw, err := json.Marshal(desiredStateRevisionCompiledRecord{
		RevisionID:          revisionID,
		ProjectID:           projectID,
		BlueprintID:         blueprintID,
		DeploymentBindingID: "bind_123",
		CommitSHA:           "abc123def456",
		TriggerKind:         "manual",
		RuntimeMode:         "standalone",
		Services: []BlueprintServiceContractRecord{
			{Name: "api", Path: "apps/api", RuntimeProfile: "service", Healthcheck: map[string]any{"path": "/healthz", "port": 8080, "protocol": "http"}},
		},
		CompatibilityPolicy: LazyopsYAMLCompatibilityPolicy{
			EnvInjection: true,
		},
		MagicDomainPolicy: LazyopsYAMLMagicDomainPolicy{
			Enabled:  true,
			Provider: "sslip.io",
		},
		ScaleToZeroPolicy: LazyopsYAMLScaleToZeroPolicy{
			Enabled: false,
		},
		PlacementAssignments: []PlacementAssignmentRecord{
			{ServiceName: "api", TargetID: "inst_123", TargetKind: "instance"},
		},
	})
	if err != nil {
		t.Fatalf("marshal compiled revision json: %v", err)
	}

	return string(raw)
}
