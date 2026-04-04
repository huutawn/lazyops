package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	RevisionStatusDraft         = "draft"
	RevisionStatusQueued        = "queued"
	RevisionStatusBuilding      = "building"
	RevisionStatusArtifactReady = "artifact_ready"
	RevisionStatusPlanned       = "planned"
	RevisionStatusApplying      = "applying"
	RevisionStatusPromoted      = "promoted"
	RevisionStatusFailed        = "failed"
	RevisionStatusRolledBack    = "rolled_back"
	RevisionStatusSuperseded    = "superseded"

	DeploymentStatusQueued         = "queued"
	DeploymentStatusRunning        = "running"
	DeploymentStatusCandidateReady = "candidate_ready"
	DeploymentStatusPromoted       = "promoted"
	DeploymentStatusFailed         = "failed"
	DeploymentStatusRolledBack     = "rolled_back"
	DeploymentStatusCanceled       = "canceled"
)

var (
	ErrBlueprintNotFound                = errors.New("blueprint not found")
	ErrRevisionNotFound                 = errors.New("revision not found")
	ErrDeploymentNotFound               = errors.New("deployment not found")
	ErrInvalidRevisionStateTransition   = errors.New("invalid revision state transition")
	ErrInvalidDeploymentStateTransition = errors.New("invalid deployment state transition")
)

type desiredStateRevisionCompiledRecord struct {
	RevisionID           string                           `json:"revision_id"`
	ProjectID            string                           `json:"project_id"`
	BlueprintID          string                           `json:"blueprint_id"`
	DeploymentBindingID  string                           `json:"deployment_binding_id"`
	CommitSHA            string                           `json:"commit_sha"`
	ArtifactRef          string                           `json:"artifact_ref,omitempty"`
	ImageRef             string                           `json:"image_ref,omitempty"`
	TriggerKind          string                           `json:"trigger_kind"`
	RuntimeMode          string                           `json:"runtime_mode"`
	Services             []BlueprintServiceContractRecord `json:"services"`
	DependencyBindings   []LazyopsYAMLDependencyBinding   `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy  LazyopsYAMLCompatibilityPolicy   `json:"compatibility_policy"`
	MagicDomainPolicy    LazyopsYAMLMagicDomainPolicy     `json:"magic_domain_policy"`
	ScaleToZeroPolicy    LazyopsYAMLScaleToZeroPolicy     `json:"scale_to_zero_policy"`
	PlacementAssignments []PlacementAssignmentRecord      `json:"placement_assignments,omitempty"`
}

type DeploymentService struct {
	projects    ProjectStore
	blueprints  BlueprintStore
	revisions   DesiredStateRevisionStore
	deployments DeploymentStore
}

func NewDeploymentService(
	projects ProjectStore,
	blueprints BlueprintStore,
	revisions DesiredStateRevisionStore,
	deployments DeploymentStore,
) *DeploymentService {
	return &DeploymentService{
		projects:    projects,
		blueprints:  blueprints,
		revisions:   revisions,
		deployments: deployments,
	}
}

func (s *DeploymentService) Create(cmd CreateDeploymentCommand) (*CreateDeploymentResult, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}

	blueprintID := strings.TrimSpace(cmd.BlueprintID)
	if blueprintID == "" {
		return nil, ErrInvalidInput
	}

	blueprint, err := s.blueprints.GetByIDForProject(project.ID, blueprintID)
	if err != nil {
		return nil, err
	}
	if blueprint == nil {
		return nil, ErrBlueprintNotFound
	}

	blueprintRecord, err := ToBlueprintRecord(*blueprint)
	if err != nil {
		return nil, err
	}

	triggerKind, err := normalizeManualDeploymentTriggerKind(cmd.TriggerKind)
	if err != nil {
		return nil, err
	}

	revisionID := utils.NewPrefixedID("rev")
	compiled := buildDesiredStateRevisionCompiledRecord(revisionID, blueprintRecord, triggerKind)
	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		return nil, err
	}

	revision := &models.DesiredStateRevision{
		ID:                   revisionID,
		ProjectID:            project.ID,
		BlueprintID:          blueprintRecord.ID,
		DeploymentBindingID:  blueprintRecord.Compiled.Binding.ID,
		CommitSHA:            compiled.CommitSHA,
		TriggerKind:          triggerKind,
		Status:               RevisionStatusQueued,
		CompiledRevisionJSON: string(compiledJSON),
	}
	if err := s.revisions.Create(revision); err != nil {
		return nil, err
	}

	deployment := &models.Deployment{
		ID:         utils.NewPrefixedID("dep"),
		ProjectID:  project.ID,
		RevisionID: revision.ID,
		Status:     DeploymentStatusQueued,
	}
	if err := s.deployments.Create(deployment); err != nil {
		return nil, err
	}

	revisionRecord, err := ToDesiredStateRevisionRecord(*revision)
	if err != nil {
		return nil, err
	}

	return &CreateDeploymentResult{
		Revision:   revisionRecord,
		Deployment: ToDeploymentRecord(*deployment),
	}, nil
}

func (s *DeploymentService) TransitionRevisionStatus(projectID, revisionID, nextStatus string) (*DesiredStateRevisionRecord, error) {
	projectID = strings.TrimSpace(projectID)
	revisionID = strings.TrimSpace(revisionID)
	if projectID == "" || revisionID == "" {
		return nil, ErrInvalidInput
	}

	next, err := normalizeRevisionStatus(nextStatus)
	if err != nil {
		return nil, err
	}

	revision, err := s.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrRevisionNotFound
	}

	current, err := normalizeRevisionStatus(revision.Status)
	if err != nil {
		return nil, err
	}
	if !canTransitionRevisionStatus(current, next) {
		return nil, fmt.Errorf("%w: %s -> %s", ErrInvalidRevisionStateTransition, current, next)
	}

	now := time.Now().UTC()
	if err := s.revisions.UpdateStatus(revision.ID, next, now); err != nil {
		return nil, err
	}
	revision.Status = next
	revision.UpdatedAt = now

	record, err := ToDesiredStateRevisionRecord(*revision)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *DeploymentService) TransitionDeploymentStatus(projectID, deploymentID, nextStatus string) (*DeploymentRecord, error) {
	projectID = strings.TrimSpace(projectID)
	deploymentID = strings.TrimSpace(deploymentID)
	if projectID == "" || deploymentID == "" {
		return nil, ErrInvalidInput
	}

	next, err := normalizeDeploymentStatus(nextStatus)
	if err != nil {
		return nil, err
	}

	deployment, err := s.deployments.GetByIDForProject(projectID, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, ErrDeploymentNotFound
	}

	current, err := normalizeDeploymentStatus(deployment.Status)
	if err != nil {
		return nil, err
	}
	if !canTransitionDeploymentStatus(current, next) {
		return nil, fmt.Errorf("%w: %s -> %s", ErrInvalidDeploymentStateTransition, current, next)
	}

	now := time.Now().UTC()
	var startedAt *time.Time
	var completedAt *time.Time
	if deployment.StartedAt != nil {
		startedAt = deployment.StartedAt
	}
	if deployment.CompletedAt != nil {
		completedAt = deployment.CompletedAt
	}
	if startedAt == nil && next != DeploymentStatusQueued {
		startedAt = &now
	}
	if isTerminalDeploymentStatus(next) {
		completedAt = &now
	}

	if err := s.deployments.UpdateStatus(deployment.ID, next, startedAt, completedAt, now); err != nil {
		return nil, err
	}
	deployment.Status = next
	deployment.StartedAt = startedAt
	deployment.CompletedAt = completedAt
	deployment.UpdatedAt = now

	record := ToDeploymentRecord(*deployment)
	return &record, nil
}

func buildDesiredStateRevisionCompiledRecord(revisionID string, blueprint BlueprintRecord, triggerKind string) desiredStateRevisionCompiledRecord {
	return desiredStateRevisionCompiledRecord{
		RevisionID:           revisionID,
		ProjectID:            blueprint.ProjectID,
		BlueprintID:          blueprint.ID,
		DeploymentBindingID:  blueprint.Compiled.Binding.ID,
		CommitSHA:            blueprint.Compiled.ArtifactMetadata.CommitSHA,
		ArtifactRef:          blueprint.Compiled.ArtifactMetadata.ArtifactRef,
		ImageRef:             blueprint.Compiled.ArtifactMetadata.ImageRef,
		TriggerKind:          triggerKind,
		RuntimeMode:          blueprint.Compiled.RuntimeMode,
		Services:             blueprint.Compiled.Services,
		DependencyBindings:   copyDependencyBindings(blueprint.Compiled.DependencyBindings),
		CompatibilityPolicy:  blueprint.Compiled.CompatibilityPolicy,
		MagicDomainPolicy:    blueprint.Compiled.MagicDomainPolicy,
		ScaleToZeroPolicy:    blueprint.Compiled.ScaleToZeroPolicy,
		PlacementAssignments: buildPlacementAssignments(blueprint.Compiled.Services, blueprint.Compiled.Binding),
	}
}

func ToDesiredStateRevisionRecord(item models.DesiredStateRevision) (DesiredStateRevisionRecord, error) {
	var compiled desiredStateRevisionCompiledRecord
	if err := json.Unmarshal([]byte(item.CompiledRevisionJSON), &compiled); err != nil {
		return DesiredStateRevisionRecord{}, err
	}
	if compiled.RevisionID == "" {
		compiled.RevisionID = item.ID
	}

	return DesiredStateRevisionRecord{
		ID:                   item.ID,
		ProjectID:            item.ProjectID,
		BlueprintID:          item.BlueprintID,
		DeploymentBindingID:  item.DeploymentBindingID,
		CommitSHA:            item.CommitSHA,
		ArtifactRef:          compiled.ArtifactRef,
		ImageRef:             compiled.ImageRef,
		TriggerKind:          item.TriggerKind,
		Status:               item.Status,
		RuntimeMode:          compiled.RuntimeMode,
		Services:             compiled.Services,
		DependencyBindings:   compiled.DependencyBindings,
		CompatibilityPolicy:  compiled.CompatibilityPolicy,
		MagicDomainPolicy:    compiled.MagicDomainPolicy,
		ScaleToZeroPolicy:    compiled.ScaleToZeroPolicy,
		PlacementAssignments: compiled.PlacementAssignments,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}, nil
}

func ToDeploymentRecord(item models.Deployment) DeploymentRecord {
	return DeploymentRecord{
		ID:          item.ID,
		ProjectID:   item.ProjectID,
		RevisionID:  item.RevisionID,
		Status:      item.Status,
		StartedAt:   item.StartedAt,
		CompletedAt: item.CompletedAt,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func normalizeManualDeploymentTriggerKind(raw string) (string, error) {
	triggerKind := strings.TrimSpace(raw)
	if triggerKind == "" {
		return "manual", nil
	}
	if strings.ContainsAny(triggerKind, " \t\r\n") {
		return "", ErrInvalidInput
	}
	return triggerKind, nil
}

func normalizeRevisionStatus(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case RevisionStatusDraft,
		RevisionStatusQueued,
		RevisionStatusBuilding,
		RevisionStatusArtifactReady,
		RevisionStatusPlanned,
		RevisionStatusApplying,
		RevisionStatusPromoted,
		RevisionStatusFailed,
		RevisionStatusRolledBack,
		RevisionStatusSuperseded:
		return strings.TrimSpace(raw), nil
	default:
		return "", ErrInvalidInput
	}
}

func normalizeDeploymentStatus(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case DeploymentStatusQueued,
		DeploymentStatusRunning,
		DeploymentStatusCandidateReady,
		DeploymentStatusPromoted,
		DeploymentStatusFailed,
		DeploymentStatusRolledBack,
		DeploymentStatusCanceled:
		return strings.TrimSpace(raw), nil
	default:
		return "", ErrInvalidInput
	}
}

func canTransitionRevisionStatus(current, next string) bool {
	if current == next {
		return true
	}

	switch current {
	case RevisionStatusDraft:
		return next == RevisionStatusQueued || next == RevisionStatusBuilding || next == RevisionStatusPlanned || next == RevisionStatusFailed || next == RevisionStatusSuperseded
	case RevisionStatusQueued:
		return next == RevisionStatusBuilding || next == RevisionStatusPlanned || next == RevisionStatusFailed || next == RevisionStatusSuperseded
	case RevisionStatusBuilding:
		return next == RevisionStatusArtifactReady || next == RevisionStatusFailed || next == RevisionStatusSuperseded
	case RevisionStatusArtifactReady:
		return next == RevisionStatusPlanned || next == RevisionStatusFailed || next == RevisionStatusSuperseded
	case RevisionStatusPlanned:
		return next == RevisionStatusApplying || next == RevisionStatusFailed || next == RevisionStatusSuperseded
	case RevisionStatusApplying:
		return next == RevisionStatusPromoted || next == RevisionStatusFailed || next == RevisionStatusRolledBack
	case RevisionStatusPromoted:
		return next == RevisionStatusRolledBack || next == RevisionStatusSuperseded
	default:
		return false
	}
}

func canTransitionDeploymentStatus(current, next string) bool {
	if current == next {
		return true
	}

	switch current {
	case DeploymentStatusQueued:
		return next == DeploymentStatusRunning || next == DeploymentStatusFailed || next == DeploymentStatusCanceled
	case DeploymentStatusRunning:
		return next == DeploymentStatusCandidateReady || next == DeploymentStatusPromoted || next == DeploymentStatusFailed || next == DeploymentStatusRolledBack || next == DeploymentStatusCanceled
	case DeploymentStatusCandidateReady:
		return next == DeploymentStatusPromoted || next == DeploymentStatusFailed || next == DeploymentStatusRolledBack || next == DeploymentStatusCanceled
	case DeploymentStatusPromoted:
		return next == DeploymentStatusRolledBack
	default:
		return false
	}
}

func isTerminalDeploymentStatus(status string) bool {
	switch status {
	case DeploymentStatusPromoted, DeploymentStatusFailed, DeploymentStatusRolledBack, DeploymentStatusCanceled:
		return true
	default:
		return false
	}
}
