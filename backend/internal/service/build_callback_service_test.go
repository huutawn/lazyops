package service

import (
	"encoding/json"
	"errors"
	"strings"
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

func TestBuildCallbackServiceAutoCompilesHiddenBlueprintWithoutExistingBlueprint(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		NamespaceSlug: "acme-api",
		RuntimeMode:   "distributed-k3s",
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
		t.Fatalf("seed service store: %v", err)
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
		DomainPolicyJSON:        `{}`,
		PlacementPolicyJSON:     `{}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	blueprintStore := newFakeBlueprintStore()
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
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","artifact_metadata_stage":{"commit_sha":"abc123def456"}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})

	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, serviceStore, blueprintStore)
	service := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, deploymentStore, buildStore, nil).
		WithServiceInventoryCompiler(compiler)

	result, err := service.Handle(BuildCallbackCommand{
		BuildJobID:  "bld_123",
		ProjectID:   "prj_123",
		CommitSHA:   "abc123def456",
		Status:      "succeeded",
		ImageRef:    "ghcr.io/lazyops/backend:abc123",
		ImageDigest: "sha256:deadbeef",
	})
	if err != nil {
		t.Fatalf("build callback with hidden compiler: %v", err)
	}
	if result.Revision == nil {
		t.Fatal("expected revision to be created from hidden service inventory blueprint")
	}
	if len(blueprintStore.items) != 1 {
		t.Fatalf("expected one hidden blueprint snapshot, got %d", len(blueprintStore.items))
	}
	if blueprintStore.items[0].SourceKind != hiddenServiceInventoryBlueprintSourceKind {
		t.Fatalf("expected hidden service inventory source kind, got %q", blueprintStore.items[0].SourceKind)
	}
	if result.Revision.BlueprintID == "" {
		t.Fatal("expected revision to keep hidden blueprint id for compatibility")
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

func TestBuildCallbackServiceAppliesSuggestedHealthcheckToAutogenDefaultService(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})

	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModelWithSingleService(
		t,
		"bp_123",
		"prj_123",
		BlueprintServiceContractRecord{
			Name:   "app",
			Path:   ".",
			Public: true,
			Healthcheck: map[string]any{
				"path":     "/",
				"port":     8080,
				"protocol": "http",
			},
		},
		"artifact://one-click/prj_123/autogen-20260412T000000Z",
	))

	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubDeliveryID:     "delivery_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoFullName:         "lazyops/frontend",
		TriggerKind:          "push",
		Status:               BuildJobStatusQueued,
		CommitSHA:            "abc123def456",
		TrackedBranch:        "main",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","project_repo_link_id":"prl_123","github_delivery_id":"delivery_123","github_installation_id":100,"github_repo_id":42,"repo_owner":"lazyops","repo_name":"frontend","repo_full_name":"lazyops/frontend","tracked_branch":"main","commit_sha":"abc123def456","trigger_kind":"push","preview_enabled":false,"artifact_metadata_stage":{"commit_sha":"abc123def456"},"retry_policy":{"max_attempts":3,"backoff":"linear"},"callback_expectation":{"path":"/api/v1/builds/callback","required_fields":["build_job_id","project_id","commit_sha","status","image_ref","image_digest","metadata.detected_services"]}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})

	service := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, deploymentStore, buildStore, nil)
	result, err := service.Handle(BuildCallbackCommand{
		BuildJobID:        "bld_123",
		ProjectID:         "prj_123",
		CommitSHA:         "abc123def456",
		Status:            "succeeded",
		ImageRef:          "ghcr.io/lazyops/frontend:abc123",
		ImageDigest:       "sha256:deadbeef",
		DetectedServices:  []string{"node"},
		DetectedFramework: "next",
		SuggestedHealthcheck: &BuildSuggestedHealthcheckRecord{
			Path: "/",
			Port: 3000,
		},
	})
	if err != nil {
		t.Fatalf("build callback success: %v", err)
	}
	if result.Revision == nil || len(result.Revision.Services) != 1 {
		t.Fatalf("expected one revision service, got %#v", result.Revision)
	}

	port := extractHealthcheckPort(result.Revision.Services[0].Healthcheck)
	if port != 3000 {
		t.Fatalf("expected overridden healthcheck port 3000, got %d", port)
	}
}

func TestBuildCallbackServiceDoesNotOverrideExplicitHealthcheck(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})

	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModelWithSingleService(
		t,
		"bp_123",
		"prj_123",
		BlueprintServiceContractRecord{
			Name:   "app",
			Path:   ".",
			Public: true,
			Healthcheck: map[string]any{
				"path":     "/health",
				"port":     5173,
				"protocol": "http",
			},
		},
		"artifact://one-click/prj_123/autogen-20260412T000000Z",
	))

	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubDeliveryID:     "delivery_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoFullName:         "lazyops/frontend",
		TriggerKind:          "push",
		Status:               BuildJobStatusQueued,
		CommitSHA:            "abc123def456",
		TrackedBranch:        "main",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","project_repo_link_id":"prl_123","github_delivery_id":"delivery_123","github_installation_id":100,"github_repo_id":42,"repo_owner":"lazyops","repo_name":"frontend","repo_full_name":"lazyops/frontend","tracked_branch":"main","commit_sha":"abc123def456","trigger_kind":"push","preview_enabled":false,"artifact_metadata_stage":{"commit_sha":"abc123def456"},"retry_policy":{"max_attempts":3,"backoff":"linear"},"callback_expectation":{"path":"/api/v1/builds/callback","required_fields":["build_job_id","project_id","commit_sha","status","image_ref","image_digest","metadata.detected_services"]}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})

	service := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, deploymentStore, buildStore, nil)
	result, err := service.Handle(BuildCallbackCommand{
		BuildJobID:        "bld_123",
		ProjectID:         "prj_123",
		CommitSHA:         "abc123def456",
		Status:            "succeeded",
		ImageRef:          "ghcr.io/lazyops/frontend:abc123",
		ImageDigest:       "sha256:deadbeef",
		DetectedServices:  []string{"node"},
		DetectedFramework: "next",
		SuggestedHealthcheck: &BuildSuggestedHealthcheckRecord{
			Path: "/",
			Port: 3000,
		},
	})
	if err != nil {
		t.Fatalf("build callback success: %v", err)
	}
	if result.Revision == nil || len(result.Revision.Services) != 1 {
		t.Fatalf("expected one revision service, got %#v", result.Revision)
	}

	port := extractHealthcheckPort(result.Revision.Services[0].Healthcheck)
	if port != 5173 {
		t.Fatalf("expected explicit healthcheck port 5173 unchanged, got %d", port)
	}
}

func TestBuildCallbackServiceDoesNotOverrideGenericFallbackForNonOneClickBlueprint(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})

	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModelWithSingleService(
		t,
		"bp_123",
		"prj_123",
		BlueprintServiceContractRecord{
			Name:   "app",
			Path:   ".",
			Public: true,
			Healthcheck: map[string]any{
				"path":     "/",
				"port":     8080,
				"protocol": "http",
			},
		},
		"artifact://builds/123",
	))

	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubDeliveryID:     "delivery_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoFullName:         "lazyops/frontend",
		TriggerKind:          "push",
		Status:               BuildJobStatusQueued,
		CommitSHA:            "abc123def456",
		TrackedBranch:        "main",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","project_repo_link_id":"prl_123","github_delivery_id":"delivery_123","github_installation_id":100,"github_repo_id":42,"repo_owner":"lazyops","repo_name":"frontend","repo_full_name":"lazyops/frontend","tracked_branch":"main","commit_sha":"abc123def456","trigger_kind":"push","preview_enabled":false,"artifact_metadata_stage":{"commit_sha":"abc123def456"},"retry_policy":{"max_attempts":3,"backoff":"linear"},"callback_expectation":{"path":"/api/v1/builds/callback","required_fields":["build_job_id","project_id","commit_sha","status","image_ref","image_digest","metadata.detected_services"]}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})

	service := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, deploymentStore, buildStore, nil)
	result, err := service.Handle(BuildCallbackCommand{
		BuildJobID:        "bld_123",
		ProjectID:         "prj_123",
		CommitSHA:         "abc123def456",
		Status:            "succeeded",
		ImageRef:          "ghcr.io/lazyops/frontend:abc123",
		ImageDigest:       "sha256:deadbeef",
		DetectedServices:  []string{"node"},
		DetectedFramework: "next",
		SuggestedHealthcheck: &BuildSuggestedHealthcheckRecord{
			Path: "/",
			Port: 3000,
		},
	})
	if err != nil {
		t.Fatalf("build callback success: %v", err)
	}
	if result.Revision == nil || len(result.Revision.Services) != 1 {
		t.Fatalf("expected one revision service, got %#v", result.Revision)
	}

	port := extractHealthcheckPort(result.Revision.Services[0].Healthcheck)
	if port != 8080 {
		t.Fatalf("expected non-one-click generic fallback healthcheck port 8080 unchanged, got %d", port)
	}
}

func TestBuildCallbackServiceAppliesArtifactToMatchedServiceInMultiServiceBlueprint(t *testing.T) {
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
		BuildJobID:           "bld_123",
		ProjectID:            "prj_123",
		CommitSHA:            "abc123def456",
		Status:               "succeeded",
		ImageRef:             "ghcr.io/lazyops/api:abc123",
		ImageDigest:          "sha256:feedface",
		DetectedServices:     []string{"api"},
		SuggestedTargetPort:  8080,
		SuggestedHealthcheck: &BuildSuggestedHealthcheckRecord{Path: "/ready", Port: 8080},
	})
	if err != nil {
		t.Fatalf("build callback success: %v", err)
	}
	if result.Revision == nil || len(result.Revision.Services) != 2 {
		t.Fatalf("expected multi-service revision, got %#v", result.Revision)
	}

	var apiSvc, webSvc *BlueprintServiceContractRecord
	for index := range result.Revision.Services {
		switch result.Revision.Services[index].Name {
		case "api":
			apiSvc = &result.Revision.Services[index]
		case "web":
			webSvc = &result.Revision.Services[index]
		}
	}
	if apiSvc == nil || webSvc == nil {
		t.Fatalf("expected api and web services, got %#v", result.Revision.Services)
	}
	if apiSvc.ImageRef != "ghcr.io/lazyops/api:abc123" || apiSvc.ImageDigest != "sha256:feedface" {
		t.Fatalf("expected api service artifact metadata to be applied, got %#v", apiSvc)
	}
	if apiSvc.ServicePort != 8080 || apiSvc.TargetPort != 8080 {
		t.Fatalf("expected api ports to be hydrated to 8080, got service=%d target=%d", apiSvc.ServicePort, apiSvc.TargetPort)
	}
	if len(result.BuildJob.ArtifactMetadata.AppliedServices) != 1 || result.BuildJob.ArtifactMetadata.AppliedServices[0] != "api" {
		t.Fatalf("expected build metadata to persist applied service api, got %#v", result.BuildJob.ArtifactMetadata)
	}
	if webSvc.ImageRef != "" || webSvc.ImageDigest != "" {
		t.Fatalf("expected unrelated service to stay untouched, got %#v", webSvc)
	}
}

func TestBuildCallbackServiceAppliesMultiServiceArtifactsFromServiceInventoryCompiler(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme Monorepo",
		Slug:          "acme-monorepo",
		NamespaceSlug: "acme-monorepo",
		RuntimeMode:   "distributed-k3s",
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{
		{Name: "api", Path: "backend", Kind: "api", Public: false},
		{Name: "web", Path: "fe", Kind: "web", Public: true},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "monorepo",
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
		DomainPolicyJSON:        `{}`,
		PlacementPolicyJSON:     `{}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubDeliveryID:     "delivery_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoFullName:         "lazyops/monorepo",
		TriggerKind:          "push",
		Status:               BuildJobStatusQueued,
		CommitSHA:            "abc123def456",
		TrackedBranch:        "main",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","artifact_metadata_stage":{"commit_sha":"abc123def456"}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})

	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, serviceStore, blueprintStore)
	service := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, deploymentStore, buildStore, nil).
		WithServiceInventoryCompiler(compiler)

	result, err := service.Handle(BuildCallbackCommand{
		BuildJobID: "bld_123",
		ProjectID:  "prj_123",
		CommitSHA:  "abc123def456",
		Status:     "succeeded",
		ServiceArtifacts: []BuildServiceArtifactRecord{
			{
				ServiceName:          "api",
				ServicePath:          "backend",
				ImageRef:             "ghcr.io/lazyops/prj_123-api:abc123",
				ImageDigest:          "sha256:api",
				SuggestedTargetPort:  8080,
				SuggestedHealthcheck: &BuildSuggestedHealthcheckRecord{Path: "/health", Port: 8080},
			},
			{
				ServiceName:          "web",
				ServicePath:          "fe",
				ImageRef:             "ghcr.io/lazyops/prj_123-web:abc123",
				ImageDigest:          "sha256:web",
				SuggestedTargetPort:  3000,
				SuggestedHealthcheck: &BuildSuggestedHealthcheckRecord{Path: "/", Port: 3000},
			},
		},
	})
	if err != nil {
		t.Fatalf("build callback multi-service success: %v", err)
	}
	if result.Revision == nil || len(result.Revision.Services) != 2 {
		t.Fatalf("expected revision with two services, got %#v", result.Revision)
	}

	serviceByName := make(map[string]BlueprintServiceContractRecord, len(result.Revision.Services))
	for _, item := range result.Revision.Services {
		serviceByName[item.Name] = item
	}
	if serviceByName["api"].ImageRef != "ghcr.io/lazyops/prj_123-api:abc123" {
		t.Fatalf("expected api image ref to be applied, got %#v", serviceByName["api"])
	}
	if serviceByName["web"].ImageRef != "ghcr.io/lazyops/prj_123-web:abc123" {
		t.Fatalf("expected web image ref to be applied, got %#v", serviceByName["web"])
	}
	if len(result.BuildJob.ArtifactMetadata.AppliedServices) != 2 {
		t.Fatalf("expected two applied services, got %#v", result.BuildJob.ArtifactMetadata)
	}
}

func TestBuildCallbackServiceRejectsMultiServiceCallbackWithoutServiceArtifacts(t *testing.T) {
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		CommitSHA:            "abc123def456",
		WorkerInputJSON:      mustBuildWorkerInputJSON(t, "bld_123", "prj_123", "abc123def456", []BuildTargetServiceRecord{{ServiceName: "api", ServicePath: "backend"}, {ServiceName: "web", ServicePath: "fe"}}),
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})
	service := NewBuildCallbackService(newFakeProjectStore(), newFakeBlueprintStore(), newFakeDesiredStateRevisionStore(), newFakeDeploymentStore(), buildStore, nil)

	_, err := service.Handle(BuildCallbackCommand{
		BuildJobID:  "bld_123",
		ProjectID:   "prj_123",
		CommitSHA:   "abc123def456",
		Status:      "succeeded",
		ImageRef:    "ghcr.io/lazyops/acme-monorepo:abc123",
		ImageDigest: "sha256:deadbeef",
	})
	if !errors.Is(err, ErrBuildArtifactMismatch) {
		t.Fatalf("expected ErrBuildArtifactMismatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "requires metadata.service_artifacts") {
		t.Fatalf("expected missing service_artifacts detail, got %v", err)
	}
}

func TestBuildCallbackServiceRejectsPartialMultiServiceCallbackArtifacts(t *testing.T) {
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		CommitSHA:            "abc123def456",
		WorkerInputJSON:      mustBuildWorkerInputJSON(t, "bld_123", "prj_123", "abc123def456", []BuildTargetServiceRecord{{ServiceName: "api", ServicePath: "backend"}, {ServiceName: "web", ServicePath: "fe"}}),
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})
	service := NewBuildCallbackService(newFakeProjectStore(), newFakeBlueprintStore(), newFakeDesiredStateRevisionStore(), newFakeDeploymentStore(), buildStore, nil)

	_, err := service.Handle(BuildCallbackCommand{
		BuildJobID: "bld_123",
		ProjectID:  "prj_123",
		CommitSHA:  "abc123def456",
		Status:     "succeeded",
		ServiceArtifacts: []BuildServiceArtifactRecord{
			{
				ServiceName: "api",
				ServicePath: "backend",
				ImageRef:    "ghcr.io/lazyops/prj_123-api:abc123",
				ImageDigest: "sha256:api",
			},
		},
	})
	if !errors.Is(err, ErrBuildArtifactMismatch) {
		t.Fatalf("expected ErrBuildArtifactMismatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "missing service_artifacts for web@fe") {
		t.Fatalf("expected missing web artifact detail, got %v", err)
	}
}

func TestBuildCallbackServiceRejectsSharedImageRefWithDifferentDigests(t *testing.T) {
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		CommitSHA:            "abc123def456",
		WorkerInputJSON:      mustBuildWorkerInputJSON(t, "bld_123", "prj_123", "abc123def456", []BuildTargetServiceRecord{{ServiceName: "api", ServicePath: "backend"}, {ServiceName: "web", ServicePath: "fe"}}),
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})
	service := NewBuildCallbackService(newFakeProjectStore(), newFakeBlueprintStore(), newFakeDesiredStateRevisionStore(), newFakeDeploymentStore(), buildStore, nil)

	_, err := service.Handle(BuildCallbackCommand{
		BuildJobID: "bld_123",
		ProjectID:  "prj_123",
		CommitSHA:  "abc123def456",
		Status:     "succeeded",
		ServiceArtifacts: []BuildServiceArtifactRecord{
			{
				ServiceName: "api",
				ServicePath: "backend",
				ImageRef:    "docker.io/tawn/prj_123:abc123",
				ImageDigest: "sha256:api",
			},
			{
				ServiceName: "web",
				ServicePath: "fe",
				ImageRef:    "docker.io/tawn/prj_123:abc123",
				ImageDigest: "sha256:web",
			},
		},
	})
	if !errors.Is(err, ErrBuildArtifactMismatch) {
		t.Fatalf("expected ErrBuildArtifactMismatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "reuse the same image_ref") {
		t.Fatalf("expected shared image_ref detail, got %v", err)
	}
}

func TestBuildCallbackServiceRejectsArtifactsThatDoNotMapToBlueprintServices(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModelWithServices(
		t,
		"bp_123",
		"prj_123",
		[]BlueprintServiceContractRecord{
			{
				Name:           "api",
				Path:           "backend",
				RuntimeProfile: "service",
				Healthcheck:    map[string]any{"path": "/healthz", "port": 8080, "protocol": "http"},
			},
			{
				Name:           "web",
				Path:           "frontend",
				Public:         true,
				RuntimeProfile: "web",
				Healthcheck:    map[string]any{"path": "/", "port": 3000, "protocol": "http"},
			},
		},
	))
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		CommitSHA:            "abc123def456",
		WorkerInputJSON:      mustBuildWorkerInputJSON(t, "bld_123", "prj_123", "abc123def456", []BuildTargetServiceRecord{{ServiceName: "api", ServicePath: "backend"}, {ServiceName: "web", ServicePath: "fe"}}),
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})
	service := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, deploymentStore, buildStore, nil)

	_, err := service.Handle(BuildCallbackCommand{
		BuildJobID: "bld_123",
		ProjectID:  "prj_123",
		CommitSHA:  "abc123def456",
		Status:     "succeeded",
		ServiceArtifacts: []BuildServiceArtifactRecord{
			{
				ServiceName: "api",
				ServicePath: "backend",
				ImageRef:    "ghcr.io/lazyops/prj_123-api:abc123",
				ImageDigest: "sha256:api",
			},
			{
				ServiceName: "web",
				ServicePath: "fe",
				ImageRef:    "ghcr.io/lazyops/prj_123-web:abc123",
				ImageDigest: "sha256:web",
			},
		},
	})
	if !errors.Is(err, ErrBuildArtifactMismatch) {
		t.Fatalf("expected ErrBuildArtifactMismatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "do not map to compiled services") {
		t.Fatalf("expected compiled service mapping detail, got %v", err)
	}
}

func mustBlueprintModelWithSingleService(
	t *testing.T,
	blueprintID string,
	projectID string,
	service BlueprintServiceContractRecord,
	artifactRef string,
) models.Blueprint {
	t.Helper()

	base := mustBlueprintModel(t, blueprintID, projectID)
	var compiled BlueprintCompiledContractRecord
	if err := json.Unmarshal([]byte(base.CompiledJSON), &compiled); err != nil {
		t.Fatalf("unmarshal base compiled blueprint: %v", err)
	}
	compiled.Services = []BlueprintServiceContractRecord{service}
	if strings.TrimSpace(artifactRef) != "" {
		compiled.ArtifactMetadata.ArtifactRef = strings.TrimSpace(artifactRef)
	}

	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		t.Fatalf("marshal compiled blueprint: %v", err)
	}
	base.CompiledJSON = string(compiledJSON)
	return base
}

func mustBlueprintModelWithServices(
	t *testing.T,
	blueprintID string,
	projectID string,
	services []BlueprintServiceContractRecord,
) models.Blueprint {
	t.Helper()

	base := mustBlueprintModel(t, blueprintID, projectID)
	var compiled BlueprintCompiledContractRecord
	if err := json.Unmarshal([]byte(base.CompiledJSON), &compiled); err != nil {
		t.Fatalf("unmarshal base compiled blueprint: %v", err)
	}
	compiled.Services = services

	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		t.Fatalf("marshal compiled blueprint: %v", err)
	}
	base.CompiledJSON = string(compiledJSON)
	return base
}

func mustBuildWorkerInputJSON(
	t *testing.T,
	buildJobID string,
	projectID string,
	commitSHA string,
	serviceTargets []BuildTargetServiceRecord,
) string {
	t.Helper()

	payload, err := json.Marshal(BuildWorkerInputRecord{
		BuildJobID:            buildJobID,
		ProjectID:             projectID,
		CommitSHA:             commitSHA,
		ServiceTargets:        serviceTargets,
		ArtifactMetadataStage: BuildArtifactMetadataStageRecord{CommitSHA: commitSHA},
		RetryPolicy: BuildRetryPolicyRecord{
			MaxAttempts: DefaultBuildJobMaxAttempts,
			Backoff:     "linear",
		},
		CallbackExpectation: BuildCallbackExpectationRecord{
			Path:           "/api/v1/builds/callback",
			RequiredFields: buildCallbackRequiredFields(serviceTargets),
		},
	})
	if err != nil {
		t.Fatalf("marshal build worker input: %v", err)
	}
	return string(payload)
}
