package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrWebhookNotConfigured    = errors.New("github webhook not configured")
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
)

type WebhookRouteResolver interface {
	LookupWebhookRoute(cmd WebhookRouteLookupCommand) (*ProjectRepoLinkRecord, error)
}

type GitHubWebhookCommand struct {
	DeliveryID string
	EventType  string
	Signature  string
	Payload    []byte
}

type GitHubWebhookNormalizedEvent struct {
	TriggerKind          string
	Action               string
	ProjectID            string
	ProjectRepoLinkID    string
	GitHubInstallationID int64
	GitHubRepoID         int64
	RepoOwner            string
	RepoName             string
	RepoFullName         string
	TrackedBranch        string
	CommitSHA            string
	PullRequestNumber    int
	PreviewEnabled       bool
	ShouldEnqueueBuild   bool
	ShouldDestroyPreview bool
}

type GitHubWebhookResult struct {
	DeliveryID    string
	EventType     string
	Status        string
	IgnoredReason string
	Event         GitHubWebhookNormalizedEvent
}

type GitHubWebhookService struct {
	webhookSecret string
	routes        WebhookRouteResolver
}

func NewGitHubWebhookService(webhookSecret string, routes WebhookRouteResolver) *GitHubWebhookService {
	return &GitHubWebhookService{
		webhookSecret: strings.TrimSpace(webhookSecret),
		routes:        routes,
	}
}

func (s *GitHubWebhookService) Handle(cmd GitHubWebhookCommand) (*GitHubWebhookResult, error) {
	if s.webhookSecret == "" {
		return nil, ErrWebhookNotConfigured
	}
	if strings.TrimSpace(cmd.Signature) == "" || !verifyGitHubWebhookSignature(s.webhookSecret, cmd.Payload, cmd.Signature) {
		return nil, ErrInvalidWebhookSignature
	}
	if strings.TrimSpace(cmd.EventType) == "" || len(cmd.Payload) == 0 {
		return nil, ErrInvalidInput
	}

	switch strings.TrimSpace(cmd.EventType) {
	case "push":
		return s.handlePush(cmd)
	case "pull_request":
		return s.handlePullRequest(cmd)
	default:
		return &GitHubWebhookResult{
			DeliveryID:    strings.TrimSpace(cmd.DeliveryID),
			EventType:     strings.TrimSpace(cmd.EventType),
			Status:        "ignored",
			IgnoredReason: "unsupported_event",
		}, nil
	}
}

type githubWebhookRepository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type githubWebhookInstallation struct {
	ID int64 `json:"id"`
}

type githubPushWebhookPayload struct {
	Ref          string                    `json:"ref"`
	After        string                    `json:"after"`
	Deleted      bool                      `json:"deleted"`
	Installation githubWebhookInstallation `json:"installation"`
	Repository   githubWebhookRepository   `json:"repository"`
}

type githubPullRequestWebhookPayload struct {
	Action       string                    `json:"action"`
	Number       int                       `json:"number"`
	Installation githubWebhookInstallation `json:"installation"`
	Repository   githubWebhookRepository   `json:"repository"`
	PullRequest  struct {
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
	} `json:"pull_request"`
}

func (s *GitHubWebhookService) handlePush(cmd GitHubWebhookCommand) (*GitHubWebhookResult, error) {
	var payload githubPushWebhookPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return nil, ErrInvalidInput
	}
	if payload.Installation.ID <= 0 || payload.Repository.ID <= 0 || strings.TrimSpace(payload.After) == "" {
		return nil, ErrInvalidInput
	}

	branch, err := normalizeTrackedBranch(payload.Ref, "")
	if err != nil {
		return ignoredWebhookResult(cmd, "unsupported_ref", GitHubWebhookNormalizedEvent{
			TriggerKind:          "push",
			Action:               "push",
			GitHubInstallationID: payload.Installation.ID,
			GitHubRepoID:         payload.Repository.ID,
			RepoOwner:            payload.Repository.Owner.Login,
			RepoName:             payload.Repository.Name,
			RepoFullName:         payload.Repository.FullName,
			CommitSHA:            payload.After,
		}), nil
	}

	event := GitHubWebhookNormalizedEvent{
		TriggerKind:          "push",
		Action:               "push",
		GitHubInstallationID: payload.Installation.ID,
		GitHubRepoID:         payload.Repository.ID,
		RepoOwner:            payload.Repository.Owner.Login,
		RepoName:             payload.Repository.Name,
		RepoFullName:         payload.Repository.FullName,
		TrackedBranch:        branch,
		CommitSHA:            payload.After,
	}
	if payload.Deleted {
		return ignoredWebhookResult(cmd, "branch_deleted", event), nil
	}

	route, err := s.lookupRoute(payload.Installation.ID, payload.Repository.ID, branch)
	if err != nil {
		if errors.Is(err, ErrRepoLinkNotFound) {
			return ignoredWebhookResult(cmd, "repo_not_linked", event), nil
		}
		return nil, err
	}

	event.ProjectID = route.ProjectID
	event.ProjectRepoLinkID = route.ID
	event.PreviewEnabled = route.PreviewEnabled
	event.RepoOwner = route.RepoOwner
	event.RepoName = route.RepoName
	event.RepoFullName = route.RepoFullName
	event.ShouldEnqueueBuild = true

	return acceptedWebhookResult(cmd, event), nil
}

func (s *GitHubWebhookService) handlePullRequest(cmd GitHubWebhookCommand) (*GitHubWebhookResult, error) {
	var payload githubPullRequestWebhookPayload
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		return nil, ErrInvalidInput
	}
	if payload.Installation.ID <= 0 || payload.Repository.ID <= 0 || payload.Number <= 0 {
		return nil, ErrInvalidInput
	}

	branch, err := normalizeTrackedBranch(payload.PullRequest.Base.Ref, "")
	if err != nil {
		return ignoredWebhookResult(cmd, "unsupported_ref", GitHubWebhookNormalizedEvent{
			TriggerKind:          "pull_request",
			Action:               payload.Action,
			GitHubInstallationID: payload.Installation.ID,
			GitHubRepoID:         payload.Repository.ID,
			RepoOwner:            payload.Repository.Owner.Login,
			RepoName:             payload.Repository.Name,
			RepoFullName:         payload.Repository.FullName,
			CommitSHA:            payload.PullRequest.Head.SHA,
			PullRequestNumber:    payload.Number,
		}), nil
	}

	triggerKind := "pull_request"
	if payload.Action == "closed" {
		triggerKind = "pull_request.closed"
	}

	event := GitHubWebhookNormalizedEvent{
		TriggerKind:          triggerKind,
		Action:               payload.Action,
		GitHubInstallationID: payload.Installation.ID,
		GitHubRepoID:         payload.Repository.ID,
		RepoOwner:            payload.Repository.Owner.Login,
		RepoName:             payload.Repository.Name,
		RepoFullName:         payload.Repository.FullName,
		TrackedBranch:        branch,
		CommitSHA:            strings.TrimSpace(payload.PullRequest.Head.SHA),
		PullRequestNumber:    payload.Number,
	}

	switch payload.Action {
	case "opened", "reopened", "synchronize", "closed":
	default:
		return ignoredWebhookResult(cmd, "unsupported_action", event), nil
	}

	route, err := s.lookupRoute(payload.Installation.ID, payload.Repository.ID, branch)
	if err != nil {
		if errors.Is(err, ErrRepoLinkNotFound) {
			return ignoredWebhookResult(cmd, "repo_not_linked", event), nil
		}
		return nil, err
	}

	event.ProjectID = route.ProjectID
	event.ProjectRepoLinkID = route.ID
	event.PreviewEnabled = route.PreviewEnabled
	event.RepoOwner = route.RepoOwner
	event.RepoName = route.RepoName
	event.RepoFullName = route.RepoFullName

	if !route.PreviewEnabled {
		return ignoredWebhookResult(cmd, "preview_disabled", event), nil
	}

	if payload.Action == "closed" {
		event.ShouldDestroyPreview = true
		return acceptedWebhookResult(cmd, event), nil
	}

	event.ShouldEnqueueBuild = true
	return acceptedWebhookResult(cmd, event), nil
}

func (s *GitHubWebhookService) lookupRoute(githubInstallationID, githubRepoID int64, trackedBranch string) (*ProjectRepoLinkRecord, error) {
	if s.routes == nil {
		return nil, ErrRepoLinkNotFound
	}
	return s.routes.LookupWebhookRoute(WebhookRouteLookupCommand{
		GitHubInstallationID: githubInstallationID,
		GitHubRepoID:         githubRepoID,
		TrackedBranch:        trackedBranch,
	})
}

func acceptedWebhookResult(cmd GitHubWebhookCommand, event GitHubWebhookNormalizedEvent) *GitHubWebhookResult {
	return &GitHubWebhookResult{
		DeliveryID: strings.TrimSpace(cmd.DeliveryID),
		EventType:  strings.TrimSpace(cmd.EventType),
		Status:     "accepted",
		Event:      event,
	}
}

func ignoredWebhookResult(cmd GitHubWebhookCommand, reason string, event GitHubWebhookNormalizedEvent) *GitHubWebhookResult {
	return &GitHubWebhookResult{
		DeliveryID:    strings.TrimSpace(cmd.DeliveryID),
		EventType:     strings.TrimSpace(cmd.EventType),
		Status:        "ignored",
		IgnoredReason: reason,
		Event:         event,
	}
}

func verifyGitHubWebhookSignature(secret string, payload []byte, signature string) bool {
	signature = strings.TrimSpace(signature)
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	received, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	return hmac.Equal(expected, received)
}

func signGitHubWebhook(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return fmt.Sprintf("sha256=%x", mac.Sum(nil))
}
