package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
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

func (f *fakeDesiredStateRevisionStore) UpdateSnapshot(revisionID, compiledRevisionJSON, manifestBundleJSON string, at time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	for _, projectItems := range f.byProjectID {
		if item, ok := projectItems[revisionID]; ok {
			if compiledRevisionJSON != "" {
				item.CompiledRevisionJSON = compiledRevisionJSON
			}
			if manifestBundleJSON != "" {
				item.ManifestBundleJSON = manifestBundleJSON
			}
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

func (f *fakeDeploymentStore) ListByProject(projectID string) ([]models.Deployment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	projectItems := f.byProjectID[projectID]
	if projectItems == nil {
		return []models.Deployment{}, nil
	}
	items := make([]models.Deployment, 0, len(projectItems))
	for _, item := range projectItems {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
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

type fakePublicURLVerifier struct {
	observations map[string]PublicURLTLSObservation
}

func (f *fakePublicURLVerifier) Observe(_ context.Context, rawURL string) PublicURLTLSObservation {
	if f != nil && f.observations != nil {
		if observation, ok := f.observations[rawURL]; ok {
			return observation
		}
	}
	return PublicURLTLSObservation{URL: rawURL, Status: publicURLStatusPending, Reason: "pending"}
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

func TestDeploymentServiceCreateAutoCompilesHiddenBlueprintWhenBlueprintIDOmitted(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		NamespaceSlug: "acme-api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:          "api",
			Path:          "apps/api",
			Kind:          "app",
			Public:        true,
			PlacementMode: servicePlacementModeSharedCluster,
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed services: %v", err)
	}
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto Primary",
		TargetRef:               "auto-primary",
		RuntimeMode:             "distributed-k3s",
		TargetKind:              "cluster",
		TargetID:                "clu_123",
		CompatibilityPolicyJSON: `{"env_injection":true}`,
		PlacementPolicyJSON:     `{}`,
		DomainPolicyJSON:        `{}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, serviceStore, blueprintStore)
	service := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	result, err := service.Create(CreateDeploymentCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		TriggerKind:     "manual",
	})
	if err != nil {
		t.Fatalf("create deployment without blueprint id: %v", err)
	}
	if result.Revision.BlueprintID == "" {
		t.Fatal("expected hidden blueprint id to persist on revision")
	}
	if len(blueprintStore.items) != 1 {
		t.Fatalf("expected hidden blueprint snapshot to be persisted, got %d", len(blueprintStore.items))
	}
	if blueprintStore.items[0].SourceKind != hiddenServiceInventoryBlueprintSourceKind {
		t.Fatalf("expected hidden blueprint source kind %q, got %q", hiddenServiceInventoryBlueprintSourceKind, blueprintStore.items[0].SourceKind)
	}
}

func TestDeploymentServiceCreateWithServiceIDsIncludesSelectedServiceAndInternalDependency(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		NamespaceSlug: "acme-api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:                    "api",
			Path:                    "apps/api",
			Kind:                    "app",
			Public:                  true,
			ConnectionTemplateKey:   "postgres.basic",
			ConnectionTargetService: "db",
		},
		{
			Name: "worker",
			Path: "apps/worker",
			Kind: "worker",
		},
		{
			Name:       "db",
			Kind:       "postgres",
			SourceType: serviceSourceTypeInternal,
			EnvBundle: map[string]string{
				"POSTGRES_DB":       "app",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "supersecret",
			},
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed services: %v", err)
	}
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto Primary",
		TargetRef:               "auto-primary",
		RuntimeMode:             "distributed-k3s",
		TargetKind:              "cluster",
		TargetID:                "clu_123",
		CompatibilityPolicyJSON: `{"env_injection":true}`,
		PlacementPolicyJSON:     `{}`,
		DomainPolicyJSON:        `{}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, serviceStore, blueprintStore)
	service := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	result, err := service.Create(CreateDeploymentCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		TriggerKind:     "manual",
		ServiceIDs:      []string{serviceModels[0].ID},
	})
	if err != nil {
		t.Fatalf("create deployment with service_ids: %v", err)
	}
	if len(result.Revision.Services) != 2 {
		t.Fatalf("expected selected service plus internal dependency, got %#v", result.Revision.Services)
	}
	names := map[string]BlueprintServiceContractRecord{}
	for _, item := range result.Revision.Services {
		names[item.Name] = item
	}
	if _, ok := names["api"]; !ok {
		t.Fatalf("expected api service in compiled revision, got %#v", result.Revision.Services)
	}
	if _, ok := names["db"]; !ok {
		t.Fatalf("expected internal postgres dependency in compiled revision, got %#v", result.Revision.Services)
	}
	if _, ok := names["worker"]; ok {
		t.Fatalf("did not expect unrelated worker service in compiled revision, got %#v", result.Revision.Services)
	}
	if names["api"].EnvBundle["DB_HOST"] != "db" {
		t.Fatalf("expected service-scoped env injection to use internal dns, got %#v", names["api"].EnvBundle)
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

func TestDeploymentServiceGetIncludesSafetyAndIncidentSummary(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123def456",
		TriggerKind:          "manual",
		Status:               RevisionStatusRolledBack,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
		CreatedAt:            time.Date(2026, 4, 9, 9, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 9, 9, 5, 0, 0, time.UTC),
	})
	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusRolledBack,
		CreatedAt:  time.Date(2026, 4, 9, 9, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 4, 9, 9, 10, 0, 0, time.UTC),
	})
	incidentStore := newFakeRuntimeIncidentStore(models.RuntimeIncident{
		ID:           "inc_123",
		ProjectID:    "prj_123",
		DeploymentID: "dep_123",
		RevisionID:   "rev_123",
		Kind:         IncidentKindUnhealthyCandidate,
		Severity:     IncidentSeverityCritical,
		Status:       IncidentStatusOpen,
		Summary:      "candidate failed health gate",
		CreatedAt:    time.Date(2026, 4, 9, 9, 6, 0, 0, time.UTC),
	})

	service := NewDeploymentService(projectStore, newFakeBlueprintStore(), revisionStore, deploymentStore).
		WithIncidentStore(incidentStore)

	record, err := service.Get("usr_123", RoleOperator, "prj_123", "dep_123")
	if err != nil {
		t.Fatalf("get deployment detail: %v", err)
	}
	if !record.SafetyPolicy.AutoRollbackEnabled {
		t.Fatal("expected auto rollback to be enabled by default")
	}
	if len(record.SafetyPolicy.Triggers) == 0 {
		t.Fatal("expected rollback triggers to be present")
	}
	if record.IncidentSummary == nil {
		t.Fatal("expected incident summary for rolled back deployment")
	}
	if record.IncidentSummary.Headline != "Deployment was auto-rolled back" {
		t.Fatalf("unexpected headline: %q", record.IncidentSummary.Headline)
	}
	if record.IncidentSummary.PrimaryAction == nil || record.IncidentSummary.PrimaryAction.ID != "retry_deployment" {
		t.Fatalf("expected retry_deployment action, got %#v", record.IncidentSummary.PrimaryAction)
	}
}

func TestDeploymentServiceGetHealthySummaryWhenNoIncident(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123def456",
		TriggerKind:          "manual",
		Status:               RevisionStatusPromoted,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
		CreatedAt:            time.Date(2026, 4, 9, 9, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 9, 9, 5, 0, 0, time.UTC),
	})
	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusPromoted,
		CreatedAt:  time.Date(2026, 4, 9, 9, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 4, 9, 9, 10, 0, 0, time.UTC),
	})
	service := NewDeploymentService(projectStore, newFakeBlueprintStore(), revisionStore, deploymentStore)

	record, err := service.Get("usr_123", RoleOperator, "prj_123", "dep_123")
	if err != nil {
		t.Fatalf("get deployment detail: %v", err)
	}
	if record.IncidentSummary == nil {
		t.Fatal("expected incident summary for healthy deployment")
	}
	if record.IncidentSummary.State != "healthy" {
		t.Fatalf("expected healthy state, got %q", record.IncidentSummary.State)
	}
}

func TestDeploymentServiceListPublishesOnlyTLSReadyPublicURLs(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Slug:   "acme-api",
	})
	compiledJSON, err := json.Marshal(desiredStateRevisionCompiledRecord{
		RevisionID:          "rev_123",
		ProjectID:           "prj_123",
		ProjectSlug:         "acme-api",
		BlueprintID:         "bp_123",
		DeploymentBindingID: "bind_123",
		RuntimeMode:         "standalone",
		Services: []BlueprintServiceContractRecord{
			{Name: "api", Public: true},
		},
	})
	if err != nil {
		t.Fatalf("marshal compiled revision: %v", err)
	}

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "manual",
		Status:               RevisionStatusPromoted,
		CompiledRevisionJSON: string(compiledJSON),
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	})
	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusPromoted,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:       "inst_123",
		UserID:   "usr_123",
		Name:     "api-host",
		PublicIP: ptrString("203.0.113.10"),
	})

	readyURL := "https://api.acme-api.203-0-113-10.sslip.io"
	fallbackURL := "https://api.acme-api.203.0.113.10.nip.io"
	service := NewDeploymentService(projectStore, newFakeBlueprintStore(), revisionStore, deploymentStore).
		WithPublicDomainSupport(bindingStore, instanceStore).
		WithPublicURLVerifier(&fakePublicURLVerifier{
			observations: map[string]PublicURLTLSObservation{
				readyURL:    {URL: readyURL, Host: "api.acme-api.203-0-113-10.sslip.io", Status: publicURLStatusReady},
				fallbackURL: {URL: fallbackURL, Host: "api.acme-api.203.0.113.10.nip.io", Status: publicURLStatusPending, Reason: "Đang chờ cấp chứng chỉ TLS công khai cho magic domain."},
			},
		})

	items, err := service.List("usr_123", RoleOperator, "prj_123")
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one deployment overview, got %d", len(items))
	}
	if got := items[0].PublicURLs; len(got) != 1 || got[0] != readyURL {
		t.Fatalf("expected only verified public url, got %#v", got)
	}
	if items[0].PublicURLStatus != publicURLStatusReady {
		t.Fatalf("expected public_url_status ready, got %q", items[0].PublicURLStatus)
	}
	if items[0].PublicURLReason != "" {
		t.Fatalf("expected empty public_url_reason when a verified url exists, got %q", items[0].PublicURLReason)
	}
}

func TestResolveStatusPublicURLsFromOverviewsPrefersReasonWhenTLSPending(t *testing.T) {
	overviews := []DeploymentOverviewRecord{
		{
			ID:              "dep_123",
			Promoted:        true,
			RolloutState:    DeploymentStatusPromoted,
			PublicURLs:      []string{},
			PublicURLStatus: publicURLStatusPending,
			PublicURLReason: "Đang chờ cấp chứng chỉ TLS công khai cho magic domain.",
		},
	}

	urls, status, reason := resolveStatusPublicURLsFromOverviews(overviews)
	if len(urls) != 0 {
		t.Fatalf("expected no public urls while TLS is pending, got %#v", urls)
	}
	if status != publicURLStatusPending {
		t.Fatalf("expected pending public url status, got %q", status)
	}
	if reason == "" {
		t.Fatal("expected pending public url reason to be preserved")
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
		ArtifactRef:         "artifact://builds/123",
		ImageRef:            "ghcr.io/lazyops/acme-api:abc123",
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
