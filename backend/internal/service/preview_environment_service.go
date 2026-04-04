package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	PreviewStatusProvisioning = "provisioning"
	PreviewStatusReady        = "ready"
	PreviewStatusDegraded     = "degraded"
	PreviewStatusDestroying   = "destroying"
	PreviewStatusDestroyed    = "destroyed"
	PreviewStatusFailed       = "failed"
)

var (
	ErrPreviewNotFound         = errors.New("preview environment not found")
	ErrPreviewAlreadyExists    = errors.New("preview environment already exists for this PR")
	ErrPreviewAlreadyDestroyed = errors.New("preview environment already destroyed")
	ErrPreviewNotEnabled       = errors.New("preview not enabled for this project")
)

type PreviewEnvironmentService struct {
	projects    ProjectStore
	repoLinks   ProjectRepoLinkStore
	revisions   DesiredStateRevisionStore
	deployments DeploymentStore
	blueprints  BlueprintStore
	previews    PreviewEnvironmentStore
	routes      PublicRouteStore
	operatorHub OperatorEventBroadcaster
}

type PreviewEnvironmentStore interface {
	Create(preview *models.PreviewEnvironment) error
	GetByIDForProject(projectID, previewID string) (*models.PreviewEnvironment, error)
	GetByPRNumber(projectID string, prNumber int) (*models.PreviewEnvironment, error)
	ListByProject(projectID string) ([]models.PreviewEnvironment, error)
	ListActiveByProject(projectID string) ([]models.PreviewEnvironment, error)
	UpdateStatus(previewID, status string, at time.Time) error
	Destroy(previewID string, reason string, at time.Time) error
}

func NewPreviewEnvironmentService(
	projects ProjectStore,
	repoLinks ProjectRepoLinkStore,
	revisions DesiredStateRevisionStore,
	deployments DeploymentStore,
	blueprints BlueprintStore,
	previews PreviewEnvironmentStore,
	routes PublicRouteStore,
	operatorHub OperatorEventBroadcaster,
) *PreviewEnvironmentService {
	return &PreviewEnvironmentService{
		projects:    projects,
		repoLinks:   repoLinks,
		revisions:   revisions,
		deployments: deployments,
		blueprints:  blueprints,
		previews:    previews,
		routes:      routes,
		operatorHub: operatorHub,
	}
}

func (s *PreviewEnvironmentService) CreateFromPR(ctx context.Context, projectID string, prNumber int, prTitle, prAuthor, commitSHA, branch string) (*PreviewRecord, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || prNumber <= 0 {
		return nil, ErrInvalidInput
	}

	repoLink, err := s.repoLinks.GetByProjectID(projectID)
	if err != nil {
		return nil, err
	}
	if repoLink == nil {
		return nil, fmt.Errorf("project %q has no repo link", projectID)
	}
	if !repoLink.PreviewEnabled {
		return nil, ErrPreviewNotEnabled
	}

	existing, err := s.previews.GetByPRNumber(projectID, prNumber)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Status != PreviewStatusDestroyed {
		return nil, ErrPreviewAlreadyExists
	}

	blueprint, err := s.blueprints.GetLatestByProject(projectID)
	if err != nil {
		return nil, err
	}
	if blueprint == nil {
		return nil, ErrBlueprintNotFound
	}

	previewID := utils.NewPrefixedID("prev")
	revisionID := utils.NewPrefixedID("rev")

	blueprintRecord, err := ToBlueprintRecord(*blueprint)
	if err != nil {
		return nil, err
	}
	blueprintRecord.Compiled.ArtifactMetadata.CommitSHA = commitSHA

	compiled := buildDesiredStateRevisionCompiledRecord(revisionID, blueprintRecord, "pull_request")
	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		return nil, err
	}

	revision := &models.DesiredStateRevision{
		ID:                   revisionID,
		ProjectID:            projectID,
		BlueprintID:          blueprint.ID,
		DeploymentBindingID:  blueprintRecord.Compiled.Binding.ID,
		CommitSHA:            commitSHA,
		TriggerKind:          "pull_request",
		Status:               RevisionStatusQueued,
		CompiledRevisionJSON: string(compiledJSON),
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	if err := s.revisions.Create(revision); err != nil {
		return nil, err
	}

	deployment := &models.Deployment{
		ID:         utils.NewPrefixedID("dep"),
		ProjectID:  projectID,
		RevisionID: revisionID,
		Status:     DeploymentStatusQueued,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.deployments.Create(deployment); err != nil {
		return nil, err
	}

	domains := s.generatePreviewDomains(repoLink.RepoOwner, repoLink.RepoName, prNumber, blueprintRecord)
	domainJSON, _ := json.Marshal(domains)

	preview := &models.PreviewEnvironment{
		ID:                previewID,
		ProjectID:         projectID,
		ProjectRepoLinkID: repoLink.ID,
		PRNumber:          prNumber,
		PRTitle:           prTitle,
		PRAuthor:          prAuthor,
		CommitSHA:         commitSHA,
		Branch:            branch,
		Status:            PreviewStatusProvisioning,
		DomainJSON:        string(domainJSON),
		RevisionID:        revisionID,
		DeploymentID:      deployment.ID,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := s.previews.Create(preview); err != nil {
		return nil, err
	}

	if s.operatorHub != nil {
		_ = s.operatorHub.BroadcastEvent("preview.created", map[string]any{
			"preview_id": previewID,
			"project_id": projectID,
			"pr_number":  prNumber,
			"commit_sha": commitSHA,
			"domains":    domains,
		})
	}

	return toPreviewRecord(*preview), nil
}

func (s *PreviewEnvironmentService) DestroyPreview(ctx context.Context, projectID, previewID, reason string) (*PreviewRecord, error) {
	preview, err := s.previews.GetByIDForProject(projectID, previewID)
	if err != nil {
		return nil, err
	}
	if preview == nil {
		return nil, ErrPreviewNotFound
	}

	if preview.Status == PreviewStatusDestroyed {
		return nil, ErrPreviewAlreadyDestroyed
	}

	now := time.Now().UTC()
	if err := s.previews.Destroy(preview.ID, reason, now); err != nil {
		return nil, err
	}

	preview.Status = PreviewStatusDestroyed
	preview.DestroyedAt = &now
	preview.DestroyReason = reason
	preview.UpdatedAt = now

	if s.operatorHub != nil {
		_ = s.operatorHub.BroadcastEvent("preview.destroyed", map[string]any{
			"preview_id":     previewID,
			"project_id":     projectID,
			"pr_number":      preview.PRNumber,
			"destroy_reason": reason,
		})
	}

	return toPreviewRecord(*preview), nil
}

func (s *PreviewEnvironmentService) DestroyPreviewByPR(ctx context.Context, projectID string, prNumber int, reason string) (*PreviewRecord, error) {
	preview, err := s.previews.GetByPRNumber(projectID, prNumber)
	if err != nil {
		return nil, err
	}
	if preview == nil {
		return nil, nil
	}

	if preview.Status == PreviewStatusDestroyed {
		return toPreviewRecord(*preview), nil
	}

	return s.DestroyPreview(ctx, projectID, preview.ID, reason)
}

func (s *PreviewEnvironmentService) CleanupStalePreviews(ctx context.Context, projectID string, maxAge time.Duration) ([]string, error) {
	active, err := s.previews.ListActiveByProject(projectID)
	if err != nil {
		return nil, err
	}

	destroyed := make([]string, 0)
	now := time.Now().UTC()

	for _, preview := range active {
		if now.Sub(preview.CreatedAt) > maxAge {
			reason := "stale_preview_auto_cleanup"
			if err := s.previews.Destroy(preview.ID, reason, now); err != nil {
				continue
			}
			destroyed = append(destroyed, preview.ID)
		}
	}

	return destroyed, nil
}

func (s *PreviewEnvironmentService) ListPreviews(projectID string) ([]PreviewRecord, error) {
	items, err := s.previews.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	out := make([]PreviewRecord, len(items))
	for i, item := range items {
		out[i] = *toPreviewRecord(item)
	}
	return out, nil
}

func (s *PreviewEnvironmentService) GetPreview(projectID, previewID string) (*PreviewRecord, error) {
	preview, err := s.previews.GetByIDForProject(projectID, previewID)
	if err != nil {
		return nil, err
	}
	if preview == nil {
		return nil, ErrPreviewNotFound
	}

	return toPreviewRecord(*preview), nil
}

func (s *PreviewEnvironmentService) UpdatePreviewStatus(projectID, previewID, status string) (*PreviewRecord, error) {
	preview, err := s.previews.GetByIDForProject(projectID, previewID)
	if err != nil {
		return nil, err
	}
	if preview == nil {
		return nil, ErrPreviewNotFound
	}

	now := time.Now().UTC()
	if err := s.previews.UpdateStatus(previewID, status, now); err != nil {
		return nil, err
	}

	preview.Status = status
	preview.UpdatedAt = now

	return toPreviewRecord(*preview), nil
}

type PreviewRecord struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"project_id"`
	PRNumber      int             `json:"pr_number"`
	PRTitle       string          `json:"pr_title"`
	PRAuthor      string          `json:"pr_author"`
	CommitSHA     string          `json:"commit_sha"`
	Branch        string          `json:"branch"`
	Status        string          `json:"status"`
	Domains       []PreviewDomain `json:"domains"`
	RevisionID    string          `json:"revision_id"`
	DeploymentID  string          `json:"deployment_id"`
	DestroyReason string          `json:"destroy_reason,omitempty"`
	DestroyedAt   *time.Time      `json:"destroyed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type PreviewDomain struct {
	ServiceName string `json:"service_name"`
	Domain      string `json:"domain"`
	HTTPS       bool   `json:"https"`
}

func toPreviewRecord(item models.PreviewEnvironment) *PreviewRecord {
	var domains []PreviewDomain
	if item.DomainJSON != "" {
		_ = json.Unmarshal([]byte(item.DomainJSON), &domains)
	}
	return &PreviewRecord{
		ID:            item.ID,
		ProjectID:     item.ProjectID,
		PRNumber:      item.PRNumber,
		PRTitle:       item.PRTitle,
		PRAuthor:      item.PRAuthor,
		CommitSHA:     item.CommitSHA,
		Branch:        item.Branch,
		Status:        item.Status,
		Domains:       domains,
		RevisionID:    item.RevisionID,
		DeploymentID:  item.DeploymentID,
		DestroyReason: item.DestroyReason,
		DestroyedAt:   item.DestroyedAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func (s *PreviewEnvironmentService) generatePreviewDomains(repoOwner, repoName string, prNumber int, blueprint BlueprintRecord) []PreviewDomain {
	baseDomain := fmt.Sprintf("pr%d-%s-%s.preview.sslip.io", prNumber, repoOwner, repoName)
	domains := make([]PreviewDomain, 0, len(blueprint.Compiled.Services))

	for _, svc := range blueprint.Compiled.Services {
		if svc.Public {
			domains = append(domains, PreviewDomain{
				ServiceName: svc.Name,
				Domain:      fmt.Sprintf("%s.%s", svc.Name, baseDomain),
				HTTPS:       true,
			})
		}
	}

	if len(domains) == 0 {
		domains = append(domains, PreviewDomain{
			ServiceName: "app",
			Domain:      baseDomain,
			HTTPS:       true,
		})
	}

	return domains
}
