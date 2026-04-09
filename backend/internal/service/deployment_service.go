package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

func (s *DeploymentService) List(requesterUserID, requesterRole, projectID string) ([]DeploymentOverviewRecord, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}

	deployments, err := s.deployments.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}
	revisions, err := s.revisions.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}

	revisionRecords, revisionNumbers, err := buildRevisionIndex(revisions)
	if err != nil {
		return nil, err
	}

	out := make([]DeploymentOverviewRecord, 0, len(deployments))
	for _, item := range deployments {
		revisionRecord, ok := revisionRecords[item.RevisionID]
		out = append(out, buildDeploymentOverview(item, revisionRecord, ok, revisionNumbers[item.RevisionID]))
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *DeploymentService) Get(requesterUserID, requesterRole, projectID, deploymentID string) (*DeploymentDetailRecord, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}

	deployment, err := s.deployments.GetByIDForProject(project.ID, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, ErrDeploymentNotFound
	}

	revisions, err := s.revisions.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}
	revisionRecords, revisionNumbers, err := buildRevisionIndex(revisions)
	if err != nil {
		return nil, err
	}

	revisionRecord, ok := revisionRecords[deployment.RevisionID]
	if !ok {
		revision, getErr := s.revisions.GetByIDForProject(project.ID, deployment.RevisionID)
		if getErr != nil {
			return nil, getErr
		}
		if revision == nil {
			return nil, ErrRevisionNotFound
		}
		parsed, parseErr := ToDesiredStateRevisionRecord(*revision)
		if parseErr != nil {
			return nil, parseErr
		}
		revisionRecord = parsed
		revisionNumbers[deployment.RevisionID] = len(revisionNumbers) + 1
	}

	overview := buildDeploymentOverview(*deployment, revisionRecord, true, revisionNumbers[deployment.RevisionID])
	detail := buildDeploymentDetail(overview, revisionRecord, *deployment)
	return &detail, nil
}

func (s *DeploymentService) Act(requesterUserID, requesterRole, projectID, deploymentID, action string) (*DeploymentDetailRecord, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}

	deployment, err := s.deployments.GetByIDForProject(project.ID, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, ErrDeploymentNotFound
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "promote":
		if _, err := s.TransitionRevisionStatus(project.ID, deployment.RevisionID, RevisionStatusPromoted); err != nil {
			return nil, err
		}
		if _, err := s.TransitionDeploymentStatus(project.ID, deployment.ID, DeploymentStatusPromoted); err != nil {
			return nil, err
		}
	case "rollback":
		if _, err := s.TransitionRevisionStatus(project.ID, deployment.RevisionID, RevisionStatusRolledBack); err != nil {
			return nil, err
		}
		if _, err := s.TransitionDeploymentStatus(project.ID, deployment.ID, DeploymentStatusRolledBack); err != nil {
			return nil, err
		}
	case "cancel":
		if _, err := s.TransitionDeploymentStatus(project.ID, deployment.ID, DeploymentStatusCanceled); err != nil {
			return nil, err
		}
		if _, revErr := s.TransitionRevisionStatus(project.ID, deployment.RevisionID, RevisionStatusFailed); revErr != nil &&
			!errors.Is(revErr, ErrInvalidRevisionStateTransition) {
			return nil, revErr
		}
	default:
		return nil, ErrInvalidInput
	}

	return s.Get(requesterUserID, requesterRole, project.ID, deployment.ID)
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

func buildRevisionIndex(revisions []models.DesiredStateRevision) (map[string]DesiredStateRevisionRecord, map[string]int, error) {
	ordered := make([]models.DesiredStateRevision, len(revisions))
	copy(ordered, revisions)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})

	records := make(map[string]DesiredStateRevisionRecord, len(ordered))
	numbers := make(map[string]int, len(ordered))
	for i, revision := range ordered {
		record, err := ToDesiredStateRevisionRecord(revision)
		if err != nil {
			return nil, nil, err
		}
		records[revision.ID] = record
		numbers[revision.ID] = i + 1
	}
	return records, numbers, nil
}

func buildDeploymentOverview(
	deployment models.Deployment,
	revision DesiredStateRevisionRecord,
	hasRevision bool,
	revisionNumber int,
) DeploymentOverviewRecord {
	buildState := RevisionStatusQueued
	rolloutState := strings.TrimSpace(deployment.Status)
	triggerKind := "manual"
	commitSHA := ""
	artifactRef := ""
	imageRef := ""
	runtimeMode := "standalone"
	services := []BlueprintServiceContractRecord{}
	placements := []PlacementAssignmentRecord{}

	if rolloutState == "" {
		rolloutState = DeploymentStatusQueued
	}

	if hasRevision {
		if strings.TrimSpace(revision.Status) != "" {
			buildState = revision.Status
		}
		if strings.TrimSpace(revision.TriggerKind) != "" {
			triggerKind = revision.TriggerKind
		}
		commitSHA = revision.CommitSHA
		artifactRef = revision.ArtifactRef
		imageRef = revision.ImageRef
		if strings.TrimSpace(revision.RuntimeMode) != "" {
			runtimeMode = revision.RuntimeMode
		}
		services = revision.Services
		placements = revision.PlacementAssignments
	}

	if revisionNumber <= 0 {
		revisionNumber = 1
	}

	promoted := rolloutState == DeploymentStatusPromoted || buildState == RevisionStatusPromoted
	return DeploymentOverviewRecord{
		ID:                   deployment.ID,
		ProjectID:            deployment.ProjectID,
		RevisionID:           deployment.RevisionID,
		Revision:             revisionNumber,
		CommitSHA:            commitSHA,
		ArtifactRef:          artifactRef,
		ImageRef:             imageRef,
		TriggerKind:          triggerKind,
		BuildState:           buildState,
		RolloutState:         rolloutState,
		Promoted:             promoted,
		TriggeredBy:          "system",
		RuntimeMode:          runtimeMode,
		Services:             services,
		PlacementAssignments: placements,
		StartedAt:            deployment.StartedAt,
		CompletedAt:          deployment.CompletedAt,
		CreatedAt:            deployment.CreatedAt,
		UpdatedAt:            deployment.UpdatedAt,
	}
}

func buildDeploymentDetail(
	overview DeploymentOverviewRecord,
	revision DesiredStateRevisionRecord,
	deployment models.Deployment,
) DeploymentDetailRecord {
	canPromote := overview.RolloutState == DeploymentStatusCandidateReady && !overview.Promoted
	canCancel := overview.RolloutState == DeploymentStatusQueued ||
		overview.RolloutState == DeploymentStatusRunning ||
		overview.RolloutState == DeploymentStatusCandidateReady
	canRollback := overview.RolloutState == DeploymentStatusPromoted || overview.BuildState == RevisionStatusPromoted

	return DeploymentDetailRecord{
		DeploymentOverviewRecord: overview,
		Timeline:                 buildDeploymentTimeline(overview, revision, deployment),
		CanRollback:              canRollback,
		CanPromote:               canPromote,
		CanCancel:                canCancel,
	}
}

func buildDeploymentTimeline(
	overview DeploymentOverviewRecord,
	revision DesiredStateRevisionRecord,
	deployment models.Deployment,
) []DeploymentTimelineEventRecord {
	events := []DeploymentTimelineEventRecord{
		{
			Timestamp:   overview.CreatedAt,
			State:       "queued",
			Label:       "Deployment queued",
			Description: "Deployment created and waiting for rollout.",
		},
	}

	revisionState := strings.TrimSpace(overview.BuildState)
	if revisionState != "" {
		timestamp := revision.UpdatedAt
		if timestamp.IsZero() {
			timestamp = overview.UpdatedAt
		}
		events = append(events, DeploymentTimelineEventRecord{
			Timestamp:   timestamp,
			State:       revisionState,
			Label:       humanizeStateLabel(revisionState),
			Description: "Revision state updated.",
		})
	}

	if deployment.StartedAt != nil {
		events = append(events, DeploymentTimelineEventRecord{
			Timestamp:   deployment.StartedAt.UTC(),
			State:       DeploymentStatusRunning,
			Label:       "Running",
			Description: "Deployment rollout started.",
		})
	}

	if rolloutState := strings.TrimSpace(overview.RolloutState); rolloutState != "" {
		events = append(events, DeploymentTimelineEventRecord{
			Timestamp:   overview.UpdatedAt,
			State:       rolloutState,
			Label:       humanizeStateLabel(rolloutState),
			Description: "Rollout state updated.",
		})
	}

	if deployment.CompletedAt != nil {
		events = append(events, DeploymentTimelineEventRecord{
			Timestamp:   deployment.CompletedAt.UTC(),
			State:       strings.TrimSpace(overview.RolloutState),
			Label:       "Completed",
			Description: "Deployment reached terminal state.",
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events
}

func humanizeStateLabel(state string) string {
	trimmed := strings.TrimSpace(state)
	if trimmed == "" {
		return "Unknown"
	}

	words := strings.Split(strings.ReplaceAll(trimmed, "_", " "), " ")
	for i := range words {
		if words[i] == "" {
			continue
		}
		words[i] = strings.ToUpper(words[i][:1]) + strings.ToLower(words[i][1:])
	}
	return strings.Join(words, " ")
}
