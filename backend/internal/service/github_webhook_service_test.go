package service

import (
	"errors"
	"fmt"
	"testing"

	"lazyops-server/internal/models"
)

type fakeWebhookRouteResolver struct {
	routes map[string]*ProjectRepoLinkRecord
	err    error
}

func newFakeWebhookRouteResolver(records ...ProjectRepoLinkRecord) *fakeWebhookRouteResolver {
	resolver := &fakeWebhookRouteResolver{
		routes: make(map[string]*ProjectRepoLinkRecord),
	}
	for _, record := range records {
		resolver.routes[resolverKey(record.GitHubInstallationID, record.GitHubRepoID, record.TrackedBranch)] = &record
	}
	return resolver
}

func (f *fakeWebhookRouteResolver) LookupWebhookRoute(cmd WebhookRouteLookupCommand) (*ProjectRepoLinkRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	if record, ok := f.routes[resolverKey(cmd.GitHubInstallationID, cmd.GitHubRepoID, cmd.TrackedBranch)]; ok {
		return record, nil
	}
	return nil, ErrRepoLinkNotFound
}

func resolverKey(installationID, repoID int64, branch string) string {
	return fmt.Sprintf("%d:%d:%s", installationID, repoID, branch)
}

func TestGitHubWebhookServiceRejectsInvalidSignature(t *testing.T) {
	service := NewGitHubWebhookService("secret", newFakeWebhookRouteResolver())
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123","installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}}}`)

	_, err := service.Handle(GitHubWebhookCommand{
		DeliveryID: "delivery_1",
		EventType:  "push",
		Signature:  "sha256=deadbeef",
		Payload:    payload,
	})
	if !errors.Is(err, ErrInvalidWebhookSignature) {
		t.Fatalf("expected ErrInvalidWebhookSignature, got %v", err)
	}
}

func TestGitHubWebhookServiceIgnoresUnlinkedRepo(t *testing.T) {
	service := NewGitHubWebhookService("secret", newFakeWebhookRouteResolver())
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123","installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}}}`)

	result, err := service.Handle(GitHubWebhookCommand{
		DeliveryID: "delivery_2",
		EventType:  "push",
		Signature:  signGitHubWebhook("secret", payload),
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	if result.Status != "ignored" || result.IgnoredReason != "repo_not_linked" {
		t.Fatalf("expected ignored repo_not_linked, got %+v", result)
	}
}

func TestGitHubWebhookServiceAcceptsPushTrigger(t *testing.T) {
	service := NewGitHubWebhookService("secret", newFakeWebhookRouteResolver(ProjectRepoLinkRecord{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
	}))
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123","installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}}}`)

	result, err := service.Handle(GitHubWebhookCommand{
		DeliveryID: "delivery_3",
		EventType:  "push",
		Signature:  signGitHubWebhook("secret", payload),
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	if result.Status != "accepted" || !result.Event.ShouldEnqueueBuild {
		t.Fatalf("expected accepted build trigger, got %+v", result)
	}
	if result.Event.ProjectID != "prj_123" || result.Event.TrackedBranch != "main" {
		t.Fatalf("expected project and branch to resolve, got %+v", result.Event)
	}
}

func TestGitHubWebhookServiceDispatchesBuildJobForAcceptedPush(t *testing.T) {
	buildDispatcher := NewBuildJobService(
		newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
			ID:                   "prl_123",
			ProjectID:            "prj_123",
			GitHubInstallationID: "ghi_alpha",
			GitHubRepoID:         42,
			RepoOwner:            "lazyops",
			RepoName:             "backend",
			TrackedBranch:        "main",
		}),
		newFakeBuildJobStore(),
	)
	service := NewGitHubWebhookService("secret", newFakeWebhookRouteResolver(ProjectRepoLinkRecord{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
	})).WithBuildDispatcher(buildDispatcher)
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123","installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}}}`)

	result, err := service.Handle(GitHubWebhookCommand{
		DeliveryID: "delivery_19",
		EventType:  "push",
		Signature:  signGitHubWebhook("secret", payload),
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	if result.BuildJob == nil {
		t.Fatalf("expected build job to be dispatched, got %+v", result)
	}
	if result.BuildJob.Status != BuildJobStatusQueued {
		t.Fatalf("expected queued build job, got %+v", result.BuildJob)
	}
}

func TestGitHubWebhookServiceIgnoresBranchPolicyRejectedBuild(t *testing.T) {
	buildDispatcher := NewBuildJobService(
		newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
			ID:                   "prl_123",
			ProjectID:            "prj_123",
			GitHubInstallationID: "ghi_alpha",
			GitHubRepoID:         42,
			RepoOwner:            "lazyops",
			RepoName:             "backend",
			TrackedBranch:        "release",
		}),
		newFakeBuildJobStore(),
	)
	service := NewGitHubWebhookService("secret", newFakeWebhookRouteResolver(ProjectRepoLinkRecord{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
	})).WithBuildDispatcher(buildDispatcher)
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123","installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}}}`)

	result, err := service.Handle(GitHubWebhookCommand{
		DeliveryID: "delivery_20",
		EventType:  "push",
		Signature:  signGitHubWebhook("secret", payload),
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	if result.Status != "ignored" || result.IgnoredReason != "branch_policy_rejected" {
		t.Fatalf("expected ignored branch_policy_rejected, got %+v", result)
	}
	if result.BuildJob != nil {
		t.Fatalf("expected no build job when branch policy rejected, got %+v", result.BuildJob)
	}
}

func TestGitHubWebhookServiceAcceptsPullRequestOpenTrigger(t *testing.T) {
	service := NewGitHubWebhookService("secret", newFakeWebhookRouteResolver(ProjectRepoLinkRecord{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
		PreviewEnabled:       true,
	}))
	payload := []byte(`{"action":"opened","number":17,"installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}},"pull_request":{"head":{"ref":"feature-x","sha":"abc123"},"base":{"ref":"main","sha":"def456"}}}`)

	result, err := service.Handle(GitHubWebhookCommand{
		DeliveryID: "delivery_4",
		EventType:  "pull_request",
		Signature:  signGitHubWebhook("secret", payload),
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	if result.Status != "accepted" || result.Event.TriggerKind != "pull_request" || !result.Event.ShouldEnqueueBuild {
		t.Fatalf("expected accepted pull_request build trigger, got %+v", result)
	}
	if result.Event.PullRequestNumber != 17 {
		t.Fatalf("expected pull request number 17, got %d", result.Event.PullRequestNumber)
	}
}

func TestGitHubWebhookServiceAcceptsPullRequestCloseTrigger(t *testing.T) {
	service := NewGitHubWebhookService("secret", newFakeWebhookRouteResolver(ProjectRepoLinkRecord{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		RepoFullName:         "lazyops/backend",
		TrackedBranch:        "main",
		PreviewEnabled:       true,
	}))
	payload := []byte(`{"action":"closed","number":17,"installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}},"pull_request":{"head":{"ref":"feature-x","sha":"abc123"},"base":{"ref":"main","sha":"def456"}}}`)

	result, err := service.Handle(GitHubWebhookCommand{
		DeliveryID: "delivery_5",
		EventType:  "pull_request",
		Signature:  signGitHubWebhook("secret", payload),
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("handle webhook: %v", err)
	}
	if result.Status != "accepted" || result.Event.TriggerKind != "pull_request.closed" || !result.Event.ShouldDestroyPreview {
		t.Fatalf("expected accepted pull_request.closed trigger, got %+v", result)
	}
}
