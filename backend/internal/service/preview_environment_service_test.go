package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakePreviewEnvironmentStore struct {
	items     []models.PreviewEnvironment
	createErr error
}

func newFakePreviewEnvironmentStore(items ...models.PreviewEnvironment) *fakePreviewEnvironmentStore {
	return &fakePreviewEnvironmentStore{items: items}
}

func (f *fakePreviewEnvironmentStore) Create(preview *models.PreviewEnvironment) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.items = append(f.items, *preview)
	return nil
}

func (f *fakePreviewEnvironmentStore) GetByIDForProject(projectID, previewID string) (*models.PreviewEnvironment, error) {
	for _, item := range f.items {
		if item.ProjectID == projectID && item.ID == previewID {
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakePreviewEnvironmentStore) GetByPRNumber(projectID string, prNumber int) (*models.PreviewEnvironment, error) {
	for _, item := range f.items {
		if item.ProjectID == projectID && item.PRNumber == prNumber {
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakePreviewEnvironmentStore) ListByProject(projectID string) ([]models.PreviewEnvironment, error) {
	out := make([]models.PreviewEnvironment, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakePreviewEnvironmentStore) ListActiveByProject(projectID string) ([]models.PreviewEnvironment, error) {
	out := make([]models.PreviewEnvironment, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID && item.Status != PreviewStatusDestroyed && item.Status != PreviewStatusFailed {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakePreviewEnvironmentStore) UpdateStatus(previewID, status string, at time.Time) error {
	for i, item := range f.items {
		if item.ID == previewID {
			f.items[i].Status = status
			f.items[i].UpdatedAt = at
			return nil
		}
	}
	return nil
}

func (f *fakePreviewEnvironmentStore) Destroy(previewID, reason string, at time.Time) error {
	for i, item := range f.items {
		if item.ID == previewID {
			f.items[i].Status = PreviewStatusDestroyed
			f.items[i].DestroyReason = reason
			f.items[i].DestroyedAt = &at
			f.items[i].UpdatedAt = at
			return nil
		}
	}
	return nil
}

func newTestPreviewService(
	projectStore ProjectStore,
	repoLinkStore ProjectRepoLinkStore,
	revisionStore DesiredStateRevisionStore,
	deploymentStore DeploymentStore,
	blueprintStore BlueprintStore,
	previewStore PreviewEnvironmentStore,
	routeStore PublicRouteStore,
	broadcaster OperatorEventBroadcaster,
) *PreviewEnvironmentService {
	return NewPreviewEnvironmentService(projectStore, repoLinkStore, revisionStore, deploymentStore, blueprintStore, previewStore, routeStore, broadcaster)
}

func TestPreviewServiceCreateFromPRSuccess(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:             "prl_123",
		ProjectID:      "prj_123",
		RepoOwner:      "lazyops",
		RepoName:       "acme-api",
		TrackedBranch:  "main",
		PreviewEnabled: true,
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	previewStore := newFakePreviewEnvironmentStore()
	broadcaster := &fakeOperatorEventBroadcaster{}

	svc := newTestPreviewService(projectStore, repoLinkStore, revisionStore, deploymentStore, blueprintStore, previewStore, newFakePublicRouteStore(), broadcaster)

	preview, err := svc.CreateFromPR(context.Background(), "prj_123", 42, "Fix login bug", "dev_user", "abc123def456", "feature/login-fix")
	if err != nil {
		t.Fatalf("create preview: %v", err)
	}

	if preview.ID == "" || preview.ID[:5] != "prev_" {
		t.Fatalf("expected prev_ prefixed id, got %q", preview.ID)
	}
	if preview.PRNumber != 42 {
		t.Fatalf("expected PR number 42, got %d", preview.PRNumber)
	}
	if preview.Status != PreviewStatusProvisioning {
		t.Fatalf("expected status provisioning, got %q", preview.Status)
	}
	if preview.CommitSHA != "abc123def456" {
		t.Fatalf("expected commit sha abc123def456, got %q", preview.CommitSHA)
	}
	if len(preview.Domains) == 0 {
		t.Fatal("expected at least one preview domain")
	}

	if len(broadcaster.events) == 0 {
		t.Fatal("expected preview.created event to be broadcast")
	}
}

func TestPreviewServiceRejectsPreviewNotEnabled(t *testing.T) {
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:             "prl_123",
		ProjectID:      "prj_123",
		RepoOwner:      "lazyops",
		RepoName:       "acme-api",
		TrackedBranch:  "main",
		PreviewEnabled: false,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		repoLinkStore,
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		newFakePreviewEnvironmentStore(),
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := svc.CreateFromPR(context.Background(), "prj_123", 42, "Test", "dev", "abc123", "main")
	if !errors.Is(err, ErrPreviewNotEnabled) {
		t.Fatalf("expected ErrPreviewNotEnabled, got %v", err)
	}
}

func TestPreviewServiceRejectsDuplicatePreview(t *testing.T) {
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:             "prl_123",
		ProjectID:      "prj_123",
		RepoOwner:      "lazyops",
		RepoName:       "acme-api",
		TrackedBranch:  "main",
		PreviewEnabled: true,
	})
	previewStore := newFakePreviewEnvironmentStore(models.PreviewEnvironment{
		ID:         "prev_existing",
		ProjectID:  "prj_123",
		PRNumber:   42,
		Status:     PreviewStatusReady,
		DomainJSON: `[{"service_name":"api","domain":"api.pr42.test.sslip.io","https":true}]`,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		repoLinkStore,
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := svc.CreateFromPR(context.Background(), "prj_123", 42, "Test", "dev", "abc123", "main")
	if !errors.Is(err, ErrPreviewAlreadyExists) {
		t.Fatalf("expected ErrPreviewAlreadyExists, got %v", err)
	}
}

func TestPreviewServiceDestroyPreviewSuccess(t *testing.T) {
	previewStore := newFakePreviewEnvironmentStore(models.PreviewEnvironment{
		ID:         "prev_123",
		ProjectID:  "prj_123",
		PRNumber:   42,
		Status:     PreviewStatusReady,
		DomainJSON: `[{"service_name":"api","domain":"api.pr42.test.sslip.io","https":true}]`,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	result, err := svc.DestroyPreview(context.Background(), "prj_123", "prev_123", "PR merged")
	if err != nil {
		t.Fatalf("destroy preview: %v", err)
	}

	if result.Status != PreviewStatusDestroyed {
		t.Fatalf("expected status destroyed, got %q", result.Status)
	}
	if result.DestroyReason != "PR merged" {
		t.Fatalf("expected destroy reason 'PR merged', got %q", result.DestroyReason)
	}
	if result.DestroyedAt == nil {
		t.Fatal("expected destroyed_at to be set")
	}
}

func TestPreviewServiceDestroyPreviewNotFound(t *testing.T) {
	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		newFakePreviewEnvironmentStore(),
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := svc.DestroyPreview(context.Background(), "prj_123", "prev_missing", "test")
	if !errors.Is(err, ErrPreviewNotFound) {
		t.Fatalf("expected ErrPreviewNotFound, got %v", err)
	}
}

func TestPreviewServiceDestroyPreviewAlreadyDestroyed(t *testing.T) {
	now := time.Now().UTC()
	previewStore := newFakePreviewEnvironmentStore(models.PreviewEnvironment{
		ID:          "prev_123",
		ProjectID:   "prj_123",
		PRNumber:    42,
		Status:      PreviewStatusDestroyed,
		DestroyedAt: &now,
		DomainJSON:  `[]`,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := svc.DestroyPreview(context.Background(), "prj_123", "prev_123", "test")
	if !errors.Is(err, ErrPreviewAlreadyDestroyed) {
		t.Fatalf("expected ErrPreviewAlreadyDestroyed, got %v", err)
	}
}

func TestPreviewServiceDestroyPreviewByPRSuccess(t *testing.T) {
	previewStore := newFakePreviewEnvironmentStore(models.PreviewEnvironment{
		ID:         "prev_123",
		ProjectID:  "prj_123",
		PRNumber:   42,
		Status:     PreviewStatusReady,
		DomainJSON: `[{"service_name":"api","domain":"api.pr42.test.sslip.io","https":true}]`,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	result, err := svc.DestroyPreviewByPR(context.Background(), "prj_123", 42, "PR closed")
	if err != nil {
		t.Fatalf("destroy preview by PR: %v", err)
	}

	if result.Status != PreviewStatusDestroyed {
		t.Fatalf("expected status destroyed, got %q", result.Status)
	}
}

func TestPreviewServiceDestroyPreviewByPRIdempotent(t *testing.T) {
	now := time.Now().UTC()
	previewStore := newFakePreviewEnvironmentStore(models.PreviewEnvironment{
		ID:          "prev_123",
		ProjectID:   "prj_123",
		PRNumber:    42,
		Status:      PreviewStatusDestroyed,
		DestroyedAt: &now,
		DomainJSON:  `[]`,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	result, err := svc.DestroyPreviewByPR(context.Background(), "prj_123", 42, "PR closed")
	if err != nil {
		t.Fatalf("destroy preview by PR idempotent: %v", err)
	}

	if result.Status != PreviewStatusDestroyed {
		t.Fatalf("expected status destroyed, got %q", result.Status)
	}
}

func TestPreviewServiceListPreviews(t *testing.T) {
	previewStore := newFakePreviewEnvironmentStore(
		models.PreviewEnvironment{
			ID:         "prev_1",
			ProjectID:  "prj_123",
			PRNumber:   42,
			Status:     PreviewStatusReady,
			DomainJSON: `[{"service_name":"api","domain":"api.pr42.test.sslip.io","https":true}]`,
			CreatedAt:  time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC),
		},
		models.PreviewEnvironment{
			ID:         "prev_2",
			ProjectID:  "prj_123",
			PRNumber:   43,
			Status:     PreviewStatusDestroyed,
			DomainJSON: `[]`,
			CreatedAt:  time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC),
		},
	)

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	previews, err := svc.ListPreviews("prj_123")
	if err != nil {
		t.Fatalf("list previews: %v", err)
	}

	if len(previews) != 2 {
		t.Fatalf("expected 2 previews, got %d", len(previews))
	}
}

func TestPreviewServiceCleanupStalePreviews(t *testing.T) {
	oldTime := time.Now().Add(-2 * time.Hour)
	previewStore := newFakePreviewEnvironmentStore(
		models.PreviewEnvironment{
			ID:         "prev_stale",
			ProjectID:  "prj_123",
			PRNumber:   42,
			Status:     PreviewStatusReady,
			DomainJSON: `[{"service_name":"api","domain":"api.pr42.test.sslip.io","https":true}]`,
			CreatedAt:  oldTime,
		},
		models.PreviewEnvironment{
			ID:         "prev_fresh",
			ProjectID:  "prj_123",
			PRNumber:   43,
			Status:     PreviewStatusReady,
			DomainJSON: `[{"service_name":"api","domain":"api.pr43.test.sslip.io","https":true}]`,
			CreatedAt:  time.Now(),
		},
	)

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	destroyed, err := svc.CleanupStalePreviews(context.Background(), "prj_123", 1*time.Hour)
	if err != nil {
		t.Fatalf("cleanup stale previews: %v", err)
	}

	if len(destroyed) != 1 {
		t.Fatalf("expected 1 stale preview destroyed, got %d", len(destroyed))
	}
	if destroyed[0] != "prev_stale" {
		t.Fatalf("expected prev_stale to be destroyed, got %q", destroyed[0])
	}
}

func TestPreviewServiceUpdatePreviewStatus(t *testing.T) {
	previewStore := newFakePreviewEnvironmentStore(models.PreviewEnvironment{
		ID:         "prev_123",
		ProjectID:  "prj_123",
		PRNumber:   42,
		Status:     PreviewStatusProvisioning,
		DomainJSON: `[{"service_name":"api","domain":"api.pr42.test.sslip.io","https":true}]`,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	result, err := svc.UpdatePreviewStatus("prj_123", "prev_123", PreviewStatusReady)
	if err != nil {
		t.Fatalf("update preview status: %v", err)
	}

	if result.Status != PreviewStatusReady {
		t.Fatalf("expected status ready, got %q", result.Status)
	}
}

func TestPreviewServiceGeneratePreviewDomains(t *testing.T) {
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		blueprintStore,
		newFakePreviewEnvironmentStore(),
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	blueprint, _ := blueprintStore.GetLatestByProject("prj_123")
	blueprintRecord, _ := ToBlueprintRecord(*blueprint)

	domains := svc.generatePreviewDomains("lazyops", "acme-api", 42, blueprintRecord)

	if len(domains) == 0 {
		t.Fatal("expected at least one preview domain")
	}

	expectedBase := "pr42-lazyops-acme-api.preview.sslip.io"
	for _, d := range domains {
		if d.ServiceName == "web" {
			expected := "web." + expectedBase
			if d.Domain != expected {
				t.Fatalf("expected domain %q for web service, got %q", expected, d.Domain)
			}
			if !d.HTTPS {
				t.Fatal("expected HTTPS to be true")
			}
		}
	}
}
