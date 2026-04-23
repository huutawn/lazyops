package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/k8sgen"
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
	registry      *runtime.Registry
	revisions     DesiredStateRevisionStore
	deployments   DeploymentStore
	incidents     RuntimeIncidentStore
	bindings      DeploymentBindingStore
	operatorHub   OperatorEventBroadcaster
	projectEnv    *ProjectEnvService
	publicDomains *PublicDomainResolver
	routing       *RoutingService
	manifestGen   *k8sgen.Generator
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
		manifestGen: k8sgen.NewGenerator(),
	}
}

func (p *RolloutPlanner) WithProjectEnvService(service *ProjectEnvService) *RolloutPlanner {
	if p == nil {
		return p
	}
	p.projectEnv = service
	return p
}

func (p *RolloutPlanner) WithPublicDomainResolver(resolver *PublicDomainResolver) *RolloutPlanner {
	if p == nil {
		return p
	}
	p.publicDomains = resolver
	return p
}

func (p *RolloutPlanner) WithRoutingService(service *RoutingService) *RolloutPlanner {
	if p == nil {
		return p
	}
	p.routing = service
	return p
}

func (p *RolloutPlanner) materializeRuntimeSnapshot(ctx context.Context, projectID string, revision *models.DesiredStateRevision, binding *models.DeploymentBinding, compiled desiredStateRevisionCompiledRecord) (desiredStateRevisionCompiledRecord, map[string]any, error) {
	if p.publicDomains != nil {
		publicDomain := p.publicDomains.Resolve(PublicDomainResolveInput{
			ProjectID:            projectID,
			ProjectSlug:          compiled.ProjectSlug,
			RuntimeMode:          binding.RuntimeMode,
			TargetKind:           binding.TargetKind,
			TargetID:             binding.TargetID,
			Services:             compiled.Services,
			PlacementAssignments: compiled.PlacementAssignments,
		})
		if len(publicDomain.Domains) > 0 {
			compiled.PublicDomains = publicDomain.Domains
		}
	}
	if p.routing != nil {
		effectiveRouting, err := p.routing.ResolveEffectiveRouting(projectID, compiled.Services, firstPublicDomainHost(compiled.PublicDomains))
		if err == nil {
			compiled.RoutingPolicy = routingPolicyRecordToLazyops(effectiveRouting)
		}
	}

	if shouldRenderK3sManifest(binding.RuntimeMode, binding.TargetKind) && p.manifestGen != nil {
		bundle, err := p.manifestGen.Generate(k8sgen.Input{
			Namespace:     firstNonEmptyCompiledValue(compiled.Namespace, revision.Namespace),
			ProjectID:     projectID,
			RevisionID:    revision.ID,
			Services:      toManifestServiceSpecs(compiled.ServiceSpecs),
			PublicDomains: toManifestDomains(compiled.PublicDomains),
			RoutingPolicy: toManifestRoutingPolicy(compiled.RoutingPolicy),
		})
		if err != nil {
			return desiredStateRevisionCompiledRecord{}, nil, fmt.Errorf("generate k3s manifest bundle: %w", err)
		}
		rollbackSource, _, rollbackErr := p.findStableRevisionSnapshot(projectID, revision.ID)
		if rollbackErr == nil && strings.TrimSpace(rollbackSource.CombinedYAML) != "" {
			bundle.RollbackYAML = rollbackSource.CombinedYAML
		} else if errors.Is(rollbackErr, ErrNoStableRevision) {
			bundle.RollbackYAML = ""
		}
		compiled.ManifestBundle = K3sManifestBundleRecord{
			Namespace:    bundle.Namespace,
			CombinedYAML: bundle.CombinedYAML,
			RollbackYAML: bundle.RollbackYAML,
			GeneratedAt:  bundle.GeneratedAt,
			Documents:    toManifestDocumentRecords(bundle.Documents),
		}
	}

	if err := p.persistRevisionSnapshot(revision, compiled); err != nil {
		return desiredStateRevisionCompiledRecord{}, nil, err
	}

	preparePayload, err := p.buildPreparePayload(ctx, projectID, revision, binding, compiled)
	if err != nil {
		return desiredStateRevisionCompiledRecord{}, nil, err
	}
	return compiled, preparePayload, nil
}

func (p *RolloutPlanner) buildPreparePayload(ctx context.Context, projectID string, revision *models.DesiredStateRevision, binding *models.DeploymentBinding, compiled desiredStateRevisionCompiledRecord) (map[string]any, error) {
	revisionPayload := map[string]any{
		"revision_id":           revision.ID,
		"project_id":            projectID,
		"project_slug":          compiled.ProjectSlug,
		"namespace":             firstNonEmptyCompiledValue(compiled.Namespace, revision.Namespace),
		"blueprint_id":          revision.BlueprintID,
		"deployment_binding_id": compiled.DeploymentBindingID,
		"commit_sha":            revision.CommitSHA,
		"artifact_ref":          compiled.ArtifactRef,
		"image_ref":             compiled.ImageRef,
		"trigger_kind":          revision.TriggerKind,
		"runtime_mode":          binding.RuntimeMode,
		"services":              compiled.Services,
		"service_specs":         compiled.ServiceSpecs,
		"dependency_bindings":   compiled.DependencyBindings,
		"internal_bindings":     compiled.InternalBindings,
		"compatibility_policy":  compiled.CompatibilityPolicy,
		"magic_domain_policy":   compiled.MagicDomainPolicy,
		"scale_to_zero_policy":  compiled.ScaleToZeroPolicy,
		"routing_policy":        compiled.RoutingPolicy,
		"placement_assignments": compiled.PlacementAssignments,
	}
	if len(compiled.PublicDomains) > 0 {
		revisionPayload["public_domains"] = toPublicDomainPayloads(compiled.PublicDomains)
	}
	if manifestBundle := toManifestBundlePayload(compiled.ManifestBundle); manifestBundle != nil {
		revisionPayload["manifest_bundle"] = manifestBundle
	}

	preparePayload := map[string]any{
		"project": map[string]any{
			"project_id": projectID,
			"slug":       compiled.ProjectSlug,
			"namespace":  firstNonEmptyCompiledValue(compiled.Namespace, revision.Namespace),
		},
		"binding": map[string]any{
			"binding_id":   binding.ID,
			"project_id":   projectID,
			"name":         binding.Name,
			"target_ref":   binding.TargetRef,
			"runtime_mode": binding.RuntimeMode,
			"target_kind":  binding.TargetKind,
			"target_id":    binding.TargetID,
		},
		"revision": revisionPayload,
	}
	if p.projectEnv != nil {
		projectEnv, err := p.projectEnv.LoadRuntimeEnv(projectID)
		if err != nil {
			return nil, fmt.Errorf("load project env: %w", err)
		}
		if len(projectEnv) > 0 {
			preparePayload["project_env"] = projectEnv
		}
	}
	return preparePayload, nil
}

func (p *RolloutPlanner) persistRevisionSnapshot(revision *models.DesiredStateRevision, compiled desiredStateRevisionCompiledRecord) error {
	if p == nil || p.revisions == nil || revision == nil {
		return nil
	}
	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		return fmt.Errorf("marshal compiled revision snapshot: %w", err)
	}
	manifestJSON, err := json.Marshal(compiled.ManifestBundle)
	if err != nil {
		return fmt.Errorf("marshal manifest bundle snapshot: %w", err)
	}
	now := time.Now().UTC()
	if err := p.revisions.UpdateSnapshot(revision.ID, string(compiledJSON), string(manifestJSON), now); err != nil {
		return fmt.Errorf("persist revision snapshot: %w", err)
	}
	revision.CompiledRevisionJSON = string(compiledJSON)
	revision.ManifestBundleJSON = string(manifestJSON)
	revision.UpdatedAt = now
	return nil
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

	compiled, preparePayload, err := p.materializeRuntimeSnapshot(ctx, projectID, revision, binding, compiled)
	if err != nil {
		return nil, err
	}

	req := runtime.RolloutRequest{
		ProjectID:       projectID,
		RevisionID:      revision.ID,
		BindingID:       binding.ID,
		RevisionPayload: preparePayload,
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

func shouldRenderK3sManifest(runtimeMode, targetKind string) bool {
	return strings.TrimSpace(runtimeMode) == runtime.RuntimeModeDistributedK3s && strings.TrimSpace(targetKind) == "cluster"
}

func toManifestServiceSpecs(items []K3sServiceSpecRecord) []k8sgen.ServiceSpec {
	if len(items) == 0 {
		return nil
	}
	out := make([]k8sgen.ServiceSpec, 0, len(items))
	for _, item := range items {
		detected := make([]k8sgen.DetectedPort, 0, len(item.DetectedPorts))
		for _, port := range item.DetectedPorts {
			detected = append(detected, k8sgen.DetectedPort{
				Port:     port.Port,
				Protocol: port.Protocol,
				Name:     port.Name,
				Exposed:  port.Exposed,
			})
		}
		out = append(out, k8sgen.ServiceSpec{
			Name:            item.Name,
			Kind:            item.Kind,
			Namespace:       item.Namespace,
			Public:          item.Public,
			PlacementMode:   item.PlacementMode,
			PlacementNodeID: item.PlacementNodeID,
			ImageRef:        item.ImageRef,
			ImageDigest:     item.ImageDigest,
			TargetPort:      item.TargetPort,
			ServicePort:     item.ServicePort,
			Replicas:        item.Replicas,
			Healthcheck:     item.Healthcheck,
			DetectedPorts:   detected,
			EnvBundle:       item.EnvBundle,
			PVCSpec:         item.PVCSpec,
			DeployStrategy:  item.DeployStrategy,
		})
	}
	return out
}

func toManifestDomains(items []PublicDomainRecord) []k8sgen.PublicDomain {
	if len(items) == 0 {
		return nil
	}
	out := make([]k8sgen.PublicDomain, 0, len(items))
	for _, item := range items {
		out = append(out, k8sgen.PublicDomain{
			ServiceName:  item.ServiceName,
			PrimaryHost:  item.PrimaryHost,
			FallbackHost: item.FallbackHost,
			PrimaryURL:   item.PrimaryURL,
			FallbackURL:  item.FallbackURL,
		})
	}
	return out
}

func toManifestRoutingPolicy(policy LazyopsYAMLRoutingPolicy) k8sgen.RoutingPolicy {
	out := k8sgen.RoutingPolicy{
		SharedDomain: strings.TrimSpace(policy.SharedDomain),
		Routes:       make([]k8sgen.RoutingRoute, 0, len(policy.Routes)),
	}
	for _, route := range policy.Routes {
		out.Routes = append(out.Routes, k8sgen.RoutingRoute{
			Path:        route.Path,
			Service:     route.Service,
			Port:        route.Port,
			WebSocket:   route.WebSocket,
			StripPrefix: route.StripPrefix,
		})
	}
	return out
}

func routingPolicyRecordToLazyops(policy RoutingPolicyRecord) LazyopsYAMLRoutingPolicy {
	out := LazyopsYAMLRoutingPolicy{
		SharedDomain: strings.TrimSpace(policy.SharedDomain),
		Routes:       make([]LazyopsYAMLRoute, 0, len(policy.Routes)),
	}
	for _, route := range policy.Routes {
		out.Routes = append(out.Routes, LazyopsYAMLRoute{
			Path:            route.Path,
			Service:         route.Service,
			Port:            route.Port,
			WebSocket:       route.WebSocket,
			StripPrefix:     route.StripPrefix,
			StripPrefixMode: route.StripPrefixMode,
		})
	}
	return out
}

func firstPublicDomainHost(items []PublicDomainRecord) string {
	for _, item := range items {
		if strings.TrimSpace(item.PrimaryHost) != "" {
			return strings.TrimSpace(item.PrimaryHost)
		}
		if strings.TrimSpace(item.FallbackHost) != "" {
			return strings.TrimSpace(item.FallbackHost)
		}
	}
	return ""
}

func toManifestDocumentRecords(items []k8sgen.ManifestDocument) []ManifestDocumentRecord {
	if len(items) == 0 {
		return nil
	}
	out := make([]ManifestDocumentRecord, 0, len(items))
	for _, item := range items {
		out = append(out, ManifestDocumentRecord{
			Name:    item.Name,
			Kind:    item.Kind,
			Path:    item.Path,
			Content: item.Content,
		})
	}
	return out
}

func toManifestBundlePayload(item K3sManifestBundleRecord) map[string]any {
	if item.Namespace == "" && item.CombinedYAML == "" && len(item.Documents) == 0 {
		return nil
	}
	docs := make([]map[string]any, 0, len(item.Documents))
	for _, doc := range item.Documents {
		docs = append(docs, map[string]any{
			"name":    doc.Name,
			"kind":    doc.Kind,
			"path":    doc.Path,
			"content": doc.Content,
		})
	}
	return map[string]any{
		"namespace":     item.Namespace,
		"combined_yaml": item.CombinedYAML,
		"rollback_yaml": item.RollbackYAML,
		"generated_at":  item.GeneratedAt,
		"documents":     docs,
	}
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

func (p *RolloutPlanner) PlanRollback(ctx context.Context, projectID, deploymentID string) (*RollbackPlan, error) {
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

	revision, err := p.revisions.GetByIDForProject(projectID, deployment.RevisionID)
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

	lastStable, err := p.findLastStableRevision(projectID, deployment.RevisionID)
	if err != nil {
		now := time.Now().UTC()
		_ = p.createIncident(projectID, deploymentID, deployment.RevisionID, IncidentKindRollbackFailure, IncidentSeverityCritical, "no stable revision found for rollback", map[string]any{
			"deployment_id": deploymentID,
			"error":         err.Error(),
		}, "", now)
		return nil, err
	}

	if !shouldRenderK3sManifest(binding.RuntimeMode, binding.TargetKind) {
		return &RollbackPlan{
			DeploymentID:       deploymentID,
			FailedRevisionID:   deployment.RevisionID,
			RestoredRevisionID: lastStable.ID,
			CommitSHA:          lastStable.CommitSHA,
			Payload: map[string]any{
				"deployment_id": deploymentID,
				"revision_id":   deployment.RevisionID,
			},
		}, nil
	}

	lastStableBundle, resolvedStableRevision, err := p.findStableRevisionSnapshot(projectID, deployment.RevisionID)
	if err != nil {
		now := time.Now().UTC()
		_ = p.createIncident(projectID, deploymentID, deployment.RevisionID, IncidentKindRollbackFailure, IncidentSeverityCritical, "stable revision snapshot is missing rollback manifest", map[string]any{
			"deployment_id":       deploymentID,
			"restored_revision":   lastStable.ID,
			"restored_commit_sha": lastStable.CommitSHA,
			"error":               err.Error(),
		}, "", now)
		return nil, err
	}
	if strings.TrimSpace(lastStableBundle.CombinedYAML) == "" {
		now := time.Now().UTC()
		_ = p.createIncident(projectID, deploymentID, deployment.RevisionID, IncidentKindRollbackFailure, IncidentSeverityCritical, "stable revision snapshot is missing rollback manifest", map[string]any{
			"deployment_id":       deploymentID,
			"restored_revision":   resolvedStableRevision.ID,
			"restored_commit_sha": resolvedStableRevision.CommitSHA,
		}, "", now)
		return nil, fmt.Errorf("stable revision %q is missing rollback manifest snapshot", resolvedStableRevision.ID)
	}

	currentManifest := compiled.ManifestBundle
	if strings.TrimSpace(currentManifest.Namespace) == "" {
		currentManifest.Namespace = lastStableBundle.Namespace
	}
	currentManifest.RollbackYAML = lastStableBundle.CombinedYAML
	compiled.ManifestBundle = currentManifest
	if err := p.persistRevisionSnapshot(revision, compiled); err != nil {
		return nil, err
	}

	preparePayload, err := p.buildPreparePayload(ctx, projectID, revision, binding, compiled)
	if err != nil {
		return nil, err
	}

	return &RollbackPlan{
		DeploymentID:       deploymentID,
		FailedRevisionID:   deployment.RevisionID,
		RestoredRevisionID: resolvedStableRevision.ID,
		CommitSHA:          resolvedStableRevision.CommitSHA,
		Payload:            preparePayload,
	}, nil
}

func (p *RolloutPlanner) FinalizeRollback(_ context.Context, projectID, deploymentID, failedRevisionID, restoredRevisionID string) (*RollbackResult, error) {
	deployment, err := p.deployments.GetByIDForProject(projectID, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, ErrDeploymentNotFound
	}

	restoredRevision, err := p.revisions.GetByIDForProject(projectID, restoredRevisionID)
	if err != nil {
		return nil, err
	}
	if restoredRevision == nil {
		return nil, ErrNoStableRevision
	}

	now := time.Now().UTC()
	if strings.TrimSpace(failedRevisionID) != "" {
		_ = p.revisions.UpdateStatus(failedRevisionID, RevisionStatusRolledBack, now)
	}
	if err := p.deployments.UpdateStatus(deploymentID, DeploymentStatusRolledBack, deployment.StartedAt, &now, now); err != nil {
		return nil, err
	}

	if p.operatorHub != nil {
		_ = p.operatorHub.BroadcastEvent(runtime.EventDeploymentRolledBack, map[string]any{
			"deployment_id":  deploymentID,
			"rolled_back_to": restoredRevision.ID,
			"project_id":     projectID,
			"commit_sha":     restoredRevision.CommitSHA,
		})
	}

	return &RollbackResult{
		DeploymentID: deploymentID,
		RolledBackTo: restoredRevision.ID,
		CommitSHA:    restoredRevision.CommitSHA,
		RolledBackAt: now,
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

	lastStable, err := p.findLastStableRevision(projectID, deployment.RevisionID)
	if err != nil {
		now := time.Now().UTC()
		_ = p.createIncident(projectID, deploymentID, deployment.RevisionID, IncidentKindRollbackFailure, IncidentSeverityCritical, "no stable revision found for rollback", map[string]any{
			"deployment_id": deploymentID,
			"error":         err.Error(),
		}, "", now)
		return nil, err
	}
	return p.FinalizeRollback(ctx, projectID, deploymentID, deployment.RevisionID, lastStable.ID)
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

func (p *RolloutPlanner) findLastStableRevision(projectID, excludeRevisionID string) (*models.DesiredStateRevision, error) {
	revisions, err := p.revisions.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	for i := len(revisions) - 1; i >= 0; i-- {
		rev := revisions[i]
		if rev.Status == RevisionStatusPromoted && strings.TrimSpace(rev.ID) != strings.TrimSpace(excludeRevisionID) {
			return &rev, nil
		}
	}

	return nil, ErrNoStableRevision
}

func (p *RolloutPlanner) findStableRevisionSnapshot(projectID, excludeRevisionID string) (K3sManifestBundleRecord, *models.DesiredStateRevision, error) {
	revision, err := p.findLastStableRevision(projectID, excludeRevisionID)
	if err != nil {
		return K3sManifestBundleRecord{}, nil, err
	}
	if revision == nil {
		return K3sManifestBundleRecord{}, nil, ErrNoStableRevision
	}
	manifest, err := resolveManifestBundleSnapshot(*revision)
	if err != nil {
		return K3sManifestBundleRecord{}, nil, err
	}
	return manifest, revision, nil
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

type RollbackPlan struct {
	DeploymentID       string
	FailedRevisionID   string
	RestoredRevisionID string
	CommitSHA          string
	Payload            map[string]any
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
