package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/internal/runtime"
	"lazyops-server/pkg/utils"
)

const (
	IncidentKindUnhealthyCandidate = "unhealthy_candidate"
	IncidentKindCrashLoop          = "crash_loop"
	IncidentKindPromotionFailure   = "promotion_failure"
	IncidentKindRollbackFailure    = "rollback_failure"
	IncidentKindHealthGateTimeout  = "health_gate_timeout"

	IncidentSeverityCritical = "critical"
	IncidentSeverityWarning  = "warning"
	IncidentSeverityInfo     = "info"

	IncidentStatusOpen         = "open"
	IncidentStatusResolved     = "resolved"
	IncidentStatusAcknowledged = "acknowledged"
)

var (
	ErrNoStableRevision          = errors.New("no stable revision found for rollback")
	ErrRollbackAlreadyRolledBack = errors.New("deployment already rolled back")
)

type RolloutPlanner struct {
	registry    *runtime.Registry
	revisions   DesiredStateRevisionStore
	deployments DeploymentStore
	incidents   RuntimeIncidentStore
	bindings    DeploymentBindingStore
	operatorHub OperatorEventBroadcaster
}

type OperatorEventBroadcaster interface {
	BroadcastEvent(eventType string, payload any) error
	BroadcastEventToUser(userID string, eventType string, payload any) error
}

func NewRolloutPlanner(
	registry *runtime.Registry,
	revisions DesiredStateRevisionStore,
	deployments DeploymentStore,
	incidents RuntimeIncidentStore,
	bindings DeploymentBindingStore,
	operatorHub OperatorEventBroadcaster,
) *RolloutPlanner {
	return &RolloutPlanner{
		registry:    registry,
		revisions:   revisions,
		deployments: deployments,
		incidents:   incidents,
		bindings:    bindings,
		operatorHub: operatorHub,
	}
}

func (p *RolloutPlanner) PlanCandidate(ctx context.Context, projectID, revisionID string) (*RolloutPlan, error) {
	revision, err := p.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrRevisionNotFound
	}

	compiled, err := parseCompiledRevision(revision.CompiledRevisionJSON)
	if err != nil {
		return nil, fmt.Errorf("parse compiled revision: %w", err)
	}

	binding, err := p.bindings.GetByIDForProject(projectID, compiled.DeploymentBindingID)
	if err != nil {
		return nil, fmt.Errorf("resolve binding: %w", err)
	}
	if binding == nil {
		return nil, fmt.Errorf("deployment binding %q not found for project %q", compiled.DeploymentBindingID, projectID)
	}

	driver, err := p.registry.Get(binding.RuntimeMode)
	if err != nil {
		return nil, fmt.Errorf("no driver for mode %q: %w", binding.RuntimeMode, err)
	}

	targetSpec := runtime.TargetSpec{
		TargetKind:  binding.TargetKind,
		TargetID:    binding.TargetID,
		RuntimeMode: binding.RuntimeMode,
	}
	if err := driver.ValidateTarget(ctx, targetSpec); err != nil {
		return nil, fmt.Errorf("target validation failed: %w", err)
	}

	revisionPayload := map[string]any{
		"revision_id":           revision.ID,
		"project_id":            projectID,
		"blueprint_id":          revision.BlueprintID,
		"deployment_binding_id": compiled.DeploymentBindingID,
		"commit_sha":            revision.CommitSHA,
		"artifact_ref":          compiled.ArtifactRef,
		"image_ref":             compiled.ImageRef,
		"trigger_kind":          revision.TriggerKind,
		"runtime_mode":          binding.RuntimeMode,
		"services":              compiled.Services,
		"dependency_bindings":   compiled.DependencyBindings,
		"compatibility_policy":  compiled.CompatibilityPolicy,
		"magic_domain_policy":   compiled.MagicDomainPolicy,
		"scale_to_zero_policy":  compiled.ScaleToZeroPolicy,
		"placement_assignments": compiled.PlacementAssignments,
	}

	req := runtime.RolloutRequest{
		ProjectID:       projectID,
		RevisionID:      revision.ID,
		BindingID:       binding.ID,
		RevisionPayload: revisionPayload,
	}

	plan, err := driver.PlanRollout(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("plan rollout: %w", err)
	}

	return &RolloutPlan{
		Steps:       plan.Steps,
		RuntimeMode: plan.RuntimeMode,
		TargetKind:  plan.TargetKind,
		RevisionID:  revision.ID,
		ProjectID:   projectID,
	}, nil
}

func (p *RolloutPlanner) ExecuteHealthGate(ctx context.Context, projectID, deploymentID, revisionID string) (*HealthGateResult, error) {
	revision, err := p.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrRevisionNotFound
	}

	compiled, err := parseCompiledRevision(revision.CompiledRevisionJSON)
	if err != nil {
		return nil, fmt.Errorf("parse compiled revision: %w", err)
	}

	result := &HealthGateResult{
		RevisionID:   revisionID,
		DeploymentID: deploymentID,
		Passed:       true,
		Services:     make([]ServiceHealthResult, 0, len(compiled.Services)),
	}

	for _, svc := range compiled.Services {
		hc := svc.Healthcheck
		if hc == nil {
			hc = map[string]any{}
		}
		result.Services = append(result.Services, ServiceHealthResult{
			ServiceName: svc.Name,
			Healthy:     true,
			Healthcheck: hc,
		})
	}

	return result, nil
}

func (p *RolloutPlanner) PromoteCandidate(ctx context.Context, projectID, deploymentID, revisionID string) (*PromotionResult, error) {
	revision, err := p.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrRevisionNotFound
	}

	if revision.Status != RevisionStatusArtifactReady && revision.Status != RevisionStatusPlanned && revision.Status != RevisionStatusApplying {
		return nil, fmt.Errorf("%w: cannot promote from status %q", ErrInvalidRevisionStateTransition, revision.Status)
	}

	now := time.Now().UTC()
	if err := p.revisions.UpdateStatus(revisionID, RevisionStatusPromoted, now); err != nil {
		return nil, err
	}

	if err := p.deployments.UpdateStatus(deploymentID, DeploymentStatusPromoted, nil, &now, now); err != nil {
		return nil, err
	}

	if p.operatorHub != nil {
		_ = p.operatorHub.BroadcastEvent(runtime.EventDeploymentPromoted, map[string]any{
			"deployment_id": deploymentID,
			"revision_id":   revisionID,
			"project_id":    projectID,
			"commit_sha":    revision.CommitSHA,
		})
	}

	return &PromotionResult{
		RevisionID:   revisionID,
		DeploymentID: deploymentID,
		PromotedAt:   now,
	}, nil
}

func (p *RolloutPlanner) RollbackDeployment(ctx context.Context, projectID, deploymentID string) (*RollbackResult, error) {
	deployment, err := p.deployments.GetByIDForProject(projectID, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, ErrDeploymentNotFound
	}

	if deployment.Status == DeploymentStatusRolledBack {
		return nil, ErrRollbackAlreadyRolledBack
	}

	lastStable, err := p.findLastStableRevision(projectID, deploymentID)
	if err != nil {
		now := time.Now().UTC()
		_ = p.createIncident(projectID, deploymentID, deployment.RevisionID, IncidentKindRollbackFailure, IncidentSeverityCritical, "no stable revision found for rollback", map[string]any{
			"deployment_id": deploymentID,
			"error":         err.Error(),
		}, "", now)
		return nil, err
	}

	now := time.Now().UTC()
	revision, err := p.revisions.GetByIDForProject(projectID, deployment.RevisionID)
	if err != nil {
		return nil, err
	}
	if revision != nil {
		_ = p.revisions.UpdateStatus(revision.ID, RevisionStatusRolledBack, now)
	}
	if err := p.deployments.UpdateStatus(deploymentID, DeploymentStatusRolledBack, deployment.StartedAt, &now, now); err != nil {
		return nil, err
	}

	if p.operatorHub != nil {
		_ = p.operatorHub.BroadcastEvent(runtime.EventDeploymentRolledBack, map[string]any{
			"deployment_id":  deploymentID,
			"rolled_back_to": lastStable.ID,
			"project_id":     projectID,
			"commit_sha":     lastStable.CommitSHA,
		})
	}

	return &RollbackResult{
		DeploymentID: deploymentID,
		RolledBackTo: lastStable.ID,
		CommitSHA:    lastStable.CommitSHA,
		RolledBackAt: now,
	}, nil
}

func (p *RolloutPlanner) RecordIncident(projectID, deploymentID, revisionID, kind, severity, summary string, details map[string]any, triggeredBy string) (*IncidentRecord, error) {
	now := time.Now().UTC()
	incident := &models.RuntimeIncident{
		ID:           utils.NewPrefixedID("inc"),
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		RevisionID:   revisionID,
		Kind:         kind,
		Severity:     severity,
		Status:       IncidentStatusOpen,
		Summary:      summary,
		TriggeredBy:  triggeredBy,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if details != nil {
		detailsJSON, err := json.Marshal(details)
		if err != nil {
			return nil, err
		}
		incident.DetailsJSON = string(detailsJSON)
	}

	if err := p.incidents.Create(incident); err != nil {
		return nil, err
	}

	if p.operatorHub != nil {
		_ = p.operatorHub.BroadcastEvent(runtime.EventIncidentCreated, map[string]any{
			"incident_id":   incident.ID,
			"project_id":    projectID,
			"deployment_id": deploymentID,
			"kind":          kind,
			"severity":      severity,
			"summary":       summary,
		})
	}

	return toIncidentRecord(*incident), nil
}

func (p *RolloutPlanner) findLastStableRevision(projectID, currentDeploymentID string) (*models.DesiredStateRevision, error) {
	revisions, err := p.revisions.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	for i := len(revisions) - 1; i >= 0; i-- {
		rev := revisions[i]
		if rev.Status == RevisionStatusPromoted {
			return &rev, nil
		}
	}

	return nil, ErrNoStableRevision
}

func (p *RolloutPlanner) createIncident(projectID, deploymentID, revisionID, kind, severity, summary string, details map[string]any, triggeredBy string, at time.Time) error {
	incident := &models.RuntimeIncident{
		ID:           utils.NewPrefixedID("inc"),
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		RevisionID:   revisionID,
		Kind:         kind,
		Severity:     severity,
		Status:       IncidentStatusOpen,
		Summary:      summary,
		TriggeredBy:  triggeredBy,
		CreatedAt:    at,
		UpdatedAt:    at,
	}

	if details != nil {
		detailsJSON, _ := json.Marshal(details)
		incident.DetailsJSON = string(detailsJSON)
	}

	return p.incidents.Create(incident)
}

type RolloutPlan struct {
	Steps       []runtime.RolloutStep
	RuntimeMode string
	TargetKind  string
	RevisionID  string
	ProjectID   string
}

type HealthGateResult struct {
	RevisionID   string
	DeploymentID string
	Passed       bool
	Services     []ServiceHealthResult
}

type ServiceHealthResult struct {
	ServiceName string
	Healthy     bool
	Healthcheck map[string]any
	Message     string
}

type PromotionResult struct {
	RevisionID   string
	DeploymentID string
	PromotedAt   time.Time
}

type RollbackResult struct {
	DeploymentID string
	RolledBackTo string
	CommitSHA    string
	RolledBackAt time.Time
}

type IncidentRecord struct {
	ID           string
	ProjectID    string
	DeploymentID string
	RevisionID   string
	Kind         string
	Severity     string
	Status       string
	Summary      string
	Details      map[string]any
	TriggeredBy  string
	ResolvedAt   *time.Time
	CreatedAt    time.Time
}

func toIncidentRecord(item models.RuntimeIncident) *IncidentRecord {
	var details map[string]any
	if item.DetailsJSON != "" {
		_ = json.Unmarshal([]byte(item.DetailsJSON), &details)
	}
	return &IncidentRecord{
		ID:           item.ID,
		ProjectID:    item.ProjectID,
		DeploymentID: item.DeploymentID,
		RevisionID:   item.RevisionID,
		Kind:         item.Kind,
		Severity:     item.Severity,
		Status:       item.Status,
		Summary:      item.Summary,
		Details:      details,
		TriggeredBy:  item.TriggeredBy,
		ResolvedAt:   item.ResolvedAt,
		CreatedAt:    item.CreatedAt,
	}
}

func parseCompiledRevision(raw string) (desiredStateRevisionCompiledRecord, error) {
	var compiled desiredStateRevisionCompiledRecord
	if err := json.Unmarshal([]byte(raw), &compiled); err != nil {
		return desiredStateRevisionCompiledRecord{}, err
	}
	return compiled, nil
}

func normalizeIncidentSeverity(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case IncidentSeverityCritical:
		return IncidentSeverityCritical
	case IncidentSeverityWarning:
		return IncidentSeverityWarning
	case IncidentSeverityInfo:
		return IncidentSeverityInfo
	default:
		return IncidentSeverityWarning
	}
}

func normalizeIncidentStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case IncidentStatusOpen:
		return IncidentStatusOpen
	case IncidentStatusResolved:
		return IncidentStatusResolved
	case IncidentStatusAcknowledged:
		return IncidentStatusAcknowledged
	default:
		return IncidentStatusOpen
	}
}
