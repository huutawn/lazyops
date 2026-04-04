package service

import (
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeBuildJobStore struct {
	byProjectID map[string]map[string]*models.BuildJob
	createErr   error
	getErr      error
	updateErr   error
}

func newFakeBuildJobStore(items ...*models.BuildJob) *fakeBuildJobStore {
	store := &fakeBuildJobStore{
		byProjectID: make(map[string]map[string]*models.BuildJob),
	}
	for _, item := range items {
		store.put(item)
	}
	return store
}

func (f *fakeBuildJobStore) Create(job *models.BuildJob) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *job
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	job.CreatedAt = cloned.CreatedAt
	job.UpdatedAt = cloned.UpdatedAt
	f.put(&cloned)
	return nil
}

func (f *fakeBuildJobStore) GetByIDForProject(projectID, buildJobID string) (*models.BuildJob, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	projectItems := f.byProjectID[projectID]
	if projectItems == nil {
		return nil, nil
	}
	if item, ok := projectItems[buildJobID]; ok {
		return item, nil
	}
	return nil, nil
}

func (f *fakeBuildJobStore) UpdateStatus(buildJobID, status string, startedAt, completedAt *time.Time, updatedAt time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	for _, projectItems := range f.byProjectID {
		if item, ok := projectItems[buildJobID]; ok {
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

func (f *fakeBuildJobStore) put(item *models.BuildJob) {
	projectItems := f.byProjectID[item.ProjectID]
	if projectItems == nil {
		projectItems = make(map[string]*models.BuildJob)
		f.byProjectID[item.ProjectID] = projectItems
	}
	cloned := *item
	projectItems[item.ID] = &cloned
}

func TestBuildJobServiceEnqueueFromWebhookSuccess(t *testing.T) {
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
		PreviewEnabled:       true,
	})
	buildStore := newFakeBuildJobStore()
	service := NewBuildJobService(repoLinkStore, buildStore)

	record, err := service.EnqueueFromWebhook("delivery_123", GitHubWebhookNormalizedEvent{
		TriggerKind:          "push",
		Action:               "push",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
		CommitSHA:            "abc123def456",
		PreviewEnabled:       true,
		ShouldEnqueueBuild:   true,
	})
	if err != nil {
		t.Fatalf("enqueue build job: %v", err)
	}

	if record.ID == "" || record.ID[:4] != "bld_" {
		t.Fatalf("expected bld_ prefixed id, got %q", record.ID)
	}
	if record.Status != BuildJobStatusQueued {
		t.Fatalf("expected queued build job, got %q", record.Status)
	}
	if record.WorkerInput.CallbackExpectation.Path != "/api/v1/builds/callback" {
		t.Fatalf("expected callback path to be staged, got %#v", record.WorkerInput.CallbackExpectation)
	}
	if record.ArtifactMetadata.CommitSHA != "abc123def456" {
		t.Fatalf("expected artifact stage commit sha abc123def456, got %q", record.ArtifactMetadata.CommitSHA)
	}
	if len(buildStore.byProjectID["prj_123"]) != 1 {
		t.Fatalf("expected one persisted build job, got %d", len(buildStore.byProjectID["prj_123"]))
	}
}

func TestBuildJobServiceRejectsBranchPolicyMismatch(t *testing.T) {
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "release",
	})
	service := NewBuildJobService(repoLinkStore, newFakeBuildJobStore())

	_, err := service.EnqueueFromWebhook("delivery_124", GitHubWebhookNormalizedEvent{
		TriggerKind:          "push",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
		CommitSHA:            "abc123def456",
		ShouldEnqueueBuild:   true,
	})
	if !errors.Is(err, ErrBuildBranchRejected) {
		t.Fatalf("expected ErrBuildBranchRejected, got %v", err)
	}
}

func TestBuildJobServicePersistsWorkerInputAndArtifactStage(t *testing.T) {
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
		PreviewEnabled:       true,
	})
	buildStore := newFakeBuildJobStore()
	service := NewBuildJobService(repoLinkStore, buildStore)

	record, err := service.EnqueueFromWebhook("delivery_125", GitHubWebhookNormalizedEvent{
		TriggerKind:          "pull_request",
		Action:               "opened",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
		CommitSHA:            "def456abc999",
		PullRequestNumber:    17,
		PreviewEnabled:       true,
		ShouldEnqueueBuild:   true,
	})
	if err != nil {
		t.Fatalf("enqueue build job: %v", err)
	}

	stored := buildStore.byProjectID["prj_123"][record.ID]
	if stored == nil {
		t.Fatalf("expected stored build job %q", record.ID)
	}
	if stored.RetryCount != 0 || stored.MaxAttempts != DefaultBuildJobMaxAttempts {
		t.Fatalf("expected retry policy 0/%d, got %d/%d", DefaultBuildJobMaxAttempts, stored.RetryCount, stored.MaxAttempts)
	}
	if record.WorkerInput.PullRequestNumber != 17 {
		t.Fatalf("expected PR number 17 in worker input, got %d", record.WorkerInput.PullRequestNumber)
	}
	if len(record.WorkerInput.CallbackExpectation.RequiredFields) == 0 {
		t.Fatalf("expected callback requirements to be staged")
	}
}
