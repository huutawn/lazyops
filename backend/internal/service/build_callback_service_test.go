package service

import (
	"errors"
	"testing"

	"lazyops-server/internal/models"
)

type fakeUserBroadcaster struct {
	userID  string
	payload any
	err     error
}

func (f *fakeUserBroadcaster) BroadcastToUser(userID string, payload any) error {
	if f.err != nil {
		return f.err
	}
	f.userID = userID
	f.payload = payload
	return nil
}

func TestBuildCallbackServiceSuccessCreatesArtifactReadyRevision(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubDeliveryID:     "delivery_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoFullName:         "lazyops/backend",
		TriggerKind:          "push",
		Status:               BuildJobStatusQueued,
		CommitSHA:            "abc123def456",
		TrackedBranch:        "main",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","project_repo_link_id":"prl_123","github_delivery_id":"delivery_123","github_installation_id":100,"github_repo_id":42,"repo_owner":"lazyops","repo_name":"backend","repo_full_name":"lazyops/backend","tracked_branch":"main","commit_sha":"abc123def456","trigger_kind":"push","preview_enabled":false,"artifact_metadata_stage":{"commit_sha":"abc123def456"},"retry_policy":{"max_attempts":3,"backoff":"linear"},"callback_expectation":{"path":"/api/v1/builds/callback","required_fields":["build_job_id","project_id","commit_sha","status","image_ref","image_digest","metadata.detected_services"]}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})
	service := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, deploymentStore, buildStore, nil)

	result, err := service.Handle(BuildCallbackCommand{
		BuildJobID:       "bld_123",
		ProjectID:        "prj_123",
		CommitSHA:        "abc123def456",
		Status:           "succeeded",
		ImageRef:         "ghcr.io/lazyops/backend:abc123",
		ImageDigest:      "sha256:deadbeef",
		DetectedServices: []string{"api", "web"},
	})
	if err != nil {
		t.Fatalf("build callback success: %v", err)
	}
	if result.BuildJob.Status != BuildJobStatusSucceeded {
		t.Fatalf("expected build job status succeeded, got %q", result.BuildJob.Status)
	}
	if result.BuildJob.ArtifactMetadata.ImageDigest != "sha256:deadbeef" {
		t.Fatalf("expected image digest to persist, got %#v", result.BuildJob.ArtifactMetadata)
	}
	if result.Revision == nil {
		t.Fatal("expected artifact-ready revision to be created")
	}
	if result.Revision.Status != RevisionStatusArtifactReady {
		t.Fatalf("expected revision status artifact_ready, got %q", result.Revision.Status)
	}
	if result.Revision.ImageRef != "ghcr.io/lazyops/backend:abc123" {
		t.Fatalf("expected revision image ref to reconcile, got %q", result.Revision.ImageRef)
	}
	if result.Deployment == nil {
		t.Fatal("expected deployment to be created")
	}
	if result.Deployment.Status != DeploymentStatusQueued {
		t.Fatalf("expected deployment status queued, got %q", result.Deployment.Status)
	}
}

func TestBuildCallbackServiceRejectsArtifactMismatch(t *testing.T) {
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		CommitSHA:            "expectedsha",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","artifact_metadata_stage":{"commit_sha":"expectedsha"},"retry_policy":{"max_attempts":3,"backoff":"linear"},"callback_expectation":{"path":"/api/v1/builds/callback"}}`,
		ArtifactMetadataJSON: `{"commit_sha":"expectedsha"}`,
	})
	service := NewBuildCallbackService(newFakeProjectStore(), newFakeBlueprintStore(), newFakeDesiredStateRevisionStore(), newFakeDeploymentStore(), buildStore, nil)

	_, err := service.Handle(BuildCallbackCommand{
		BuildJobID: "bld_123",
		ProjectID:  "prj_123",
		CommitSHA:  "actualsha",
		Status:     "failed",
	})
	if !errors.Is(err, ErrBuildArtifactMismatch) {
		t.Fatalf("expected ErrBuildArtifactMismatch, got %v", err)
	}
}

func TestBuildCallbackServiceRejectsUnknownBuildJob(t *testing.T) {
	service := NewBuildCallbackService(newFakeProjectStore(), newFakeBlueprintStore(), newFakeDesiredStateRevisionStore(), newFakeDeploymentStore(), newFakeBuildJobStore(), nil)

	_, err := service.Handle(BuildCallbackCommand{
		BuildJobID: "bld_missing",
		ProjectID:  "prj_123",
		CommitSHA:  "abc123",
		Status:     "failed",
	})
	if !errors.Is(err, ErrBuildJobNotFound) {
		t.Fatalf("expected ErrBuildJobNotFound, got %v", err)
	}
}

func TestBuildCallbackServiceBroadcastsFailureEvent(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubDeliveryID:     "delivery_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoFullName:         "lazyops/backend",
		TriggerKind:          "push",
		Status:               BuildJobStatusQueued,
		CommitSHA:            "abc123def456",
		TrackedBranch:        "main",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","project_repo_link_id":"prl_123","github_delivery_id":"delivery_123","github_installation_id":100,"github_repo_id":42,"repo_owner":"lazyops","repo_name":"backend","repo_full_name":"lazyops/backend","tracked_branch":"main","commit_sha":"abc123def456","trigger_kind":"push","preview_enabled":false,"artifact_metadata_stage":{"commit_sha":"abc123def456"},"retry_policy":{"max_attempts":3,"backoff":"linear"},"callback_expectation":{"path":"/api/v1/builds/callback","required_fields":["build_job_id","project_id","commit_sha","status","image_ref","image_digest","metadata.detected_services"]}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})
	broadcaster := new(fakeUserBroadcaster)
	service := NewBuildCallbackService(projectStore, newFakeBlueprintStore(), newFakeDesiredStateRevisionStore(), newFakeDeploymentStore(), buildStore, broadcaster)

	result, err := service.Handle(BuildCallbackCommand{
		BuildJobID: "bld_123",
		ProjectID:  "prj_123",
		CommitSHA:  "abc123def456",
		Status:     "failed",
	})
	if err != nil {
		t.Fatalf("build callback failure: %v", err)
	}
	if result.BuildJob.Status != BuildJobStatusFailed {
		t.Fatalf("expected failed build status, got %q", result.BuildJob.Status)
	}
	if broadcaster.userID != "usr_123" {
		t.Fatalf("expected failure event for usr_123, got %q", broadcaster.userID)
	}
	event, ok := broadcaster.payload.(BuildRealtimeEvent)
	if !ok {
		t.Fatalf("expected BuildRealtimeEvent payload, got %#v", broadcaster.payload)
	}
	if event.Type != "build.job.failed" {
		t.Fatalf("expected build.job.failed event, got %#v", event)
	}
}
