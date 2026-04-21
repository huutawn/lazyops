package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/logger"
	"lazyops-server/pkg/utils"
)

var ErrBuildBranchRejected = errors.New("build branch rejected")

type BuildJobService struct {
	repoLinks ProjectRepoLinkStore
	buildJobs BuildJobStore
	services  ProjectServiceStore
}

func NewBuildJobService(repoLinks ProjectRepoLinkStore, buildJobs BuildJobStore) *BuildJobService {
	return &BuildJobService{
		repoLinks: repoLinks,
		buildJobs: buildJobs,
	}
}

func (s *BuildJobService) WithServiceStore(store ProjectServiceStore) *BuildJobService {
	if s == nil {
		return s
	}
	s.services = store
	return s
}

func (s *BuildJobService) EnqueueFromWebhook(deliveryID string, event GitHubWebhookNormalizedEvent) (*BuildJobRecord, error) {
	if s.buildJobs == nil || s.repoLinks == nil {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(deliveryID) == "" || strings.TrimSpace(event.ProjectID) == "" || strings.TrimSpace(event.ProjectRepoLinkID) == "" {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(event.CommitSHA) == "" || strings.TrimSpace(event.TrackedBranch) == "" || strings.TrimSpace(event.TriggerKind) == "" {
		return nil, ErrInvalidInput
	}
	if !event.ShouldEnqueueBuild {
		return nil, ErrInvalidInput
	}

	existing, err := s.buildJobs.GetByDeliveryID(strings.TrimSpace(deliveryID))
	if err != nil {
		return nil, err
	}
	if existing != nil {
		record, err := ToBuildJobRecord(*existing)
		if err != nil {
			return nil, err
		}
		return &record, nil
	}

	link, err := s.repoLinks.GetByProjectID(event.ProjectID)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, ErrRepoLinkNotFound
	}
	if link.ID != event.ProjectRepoLinkID ||
		link.GitHubRepoID != event.GitHubRepoID ||
		link.TrackedBranch != event.TrackedBranch ||
		link.RepoOwner != event.RepoOwner ||
		link.RepoName != event.RepoName {
		return nil, ErrBuildBranchRejected
	}

	return s.enqueueBuildJob(buildJobEnqueueSpec{
		ProjectID:            event.ProjectID,
		ProjectRepoLinkID:    event.ProjectRepoLinkID,
		GitHubDeliveryID:     strings.TrimSpace(deliveryID),
		GitHubInstallationID: event.GitHubInstallationID,
		GitHubRepoID:         event.GitHubRepoID,
		RepoOwner:            event.RepoOwner,
		RepoName:             event.RepoName,
		RepoFullName:         event.RepoFullName,
		TrackedBranch:        event.TrackedBranch,
		CommitSHA:            strings.TrimSpace(event.CommitSHA),
		TriggerKind:          event.TriggerKind,
		PullRequestNumber:    event.PullRequestNumber,
		PreviewEnabled:       event.PreviewEnabled,
	})
}

func (s *BuildJobService) EnqueueManual(cmd ManualBuildEnqueueCommand) (*BuildJobRecord, error) {
	if s == nil || s.buildJobs == nil {
		return nil, ErrInvalidInput
	}
	projectID := strings.TrimSpace(cmd.ProjectID)
	projectRepoLinkID := strings.TrimSpace(cmd.ProjectRepoLinkID)
	trackedBranch := strings.TrimSpace(cmd.TrackedBranch)
	if projectID == "" || projectRepoLinkID == "" || trackedBranch == "" {
		return nil, ErrInvalidInput
	}
	if cmd.GitHubInstallationID <= 0 || cmd.GitHubRepoID <= 0 {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(cmd.RepoOwner) == "" || strings.TrimSpace(cmd.RepoName) == "" || strings.TrimSpace(cmd.RepoFullName) == "" {
		return nil, ErrInvalidInput
	}

	triggerKind := strings.TrimSpace(cmd.TriggerKind)
	if triggerKind == "" {
		triggerKind = "manual"
	}

	jobID := utils.NewPrefixedID("bld")
	return s.enqueueBuildJob(buildJobEnqueueSpec{
		JobID:                jobID,
		ProjectID:            projectID,
		ProjectRepoLinkID:    projectRepoLinkID,
		GitHubDeliveryID:     fmt.Sprintf("manual:%s", jobID),
		GitHubInstallationID: cmd.GitHubInstallationID,
		GitHubRepoID:         cmd.GitHubRepoID,
		RepoOwner:            strings.TrimSpace(cmd.RepoOwner),
		RepoName:             strings.TrimSpace(cmd.RepoName),
		RepoFullName:         strings.TrimSpace(cmd.RepoFullName),
		TrackedBranch:        trackedBranch,
		CommitSHA:            strings.TrimSpace(cmd.CommitSHA),
		TriggerKind:          triggerKind,
		PreviewEnabled:       cmd.PreviewEnabled,
	})
}

func (s *BuildJobService) resolveBuildTargets(projectID string) ([]BuildTargetServiceRecord, error) {
	if s == nil || s.services == nil {
		return nil, nil
	}
	items, err := s.services.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	targets := make([]BuildTargetServiceRecord, 0, len(items))
	for _, item := range items {
		record, err := ToProjectServiceRecord(item)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(record.SourceType) != serviceSourceTypeRepo {
			continue
		}
		targets = append(targets, BuildTargetServiceRecord{
			ServiceName:         record.Name,
			ServicePath:         record.Path,
			RuntimeProfile:      record.RuntimeProfile,
			Public:              record.Public,
			StartHint:           record.StartHint,
			DeclaredTargetPort:  record.TargetPort,
			DeclaredServicePort: record.ServicePort,
			DeclaredHealthcheck: cloneAnyMap(record.Healthcheck),
		})
	}
	return targets, nil
}

type buildJobEnqueueSpec struct {
	JobID                string
	ProjectID            string
	ProjectRepoLinkID    string
	GitHubDeliveryID     string
	GitHubInstallationID int64
	GitHubRepoID         int64
	RepoOwner            string
	RepoName             string
	RepoFullName         string
	TrackedBranch        string
	CommitSHA            string
	TriggerKind          string
	PullRequestNumber    int
	PreviewEnabled       bool
}

func (s *BuildJobService) enqueueBuildJob(spec buildJobEnqueueSpec) (*BuildJobRecord, error) {
	if s == nil || s.buildJobs == nil {
		return nil, ErrInvalidInput
	}

	jobID := strings.TrimSpace(spec.JobID)
	if jobID == "" {
		jobID = utils.NewPrefixedID("bld")
	}

	artifactStage := BuildArtifactMetadataStageRecord{
		CommitSHA: strings.TrimSpace(spec.CommitSHA),
	}
	serviceTargets, err := s.resolveBuildTargets(spec.ProjectID)
	if err != nil {
		return nil, err
	}
	serviceTargets = normalizeBuildTargetServices(serviceTargets)
	workerInput := BuildWorkerInputRecord{
		BuildJobID:            jobID,
		ProjectID:             strings.TrimSpace(spec.ProjectID),
		ProjectRepoLinkID:     strings.TrimSpace(spec.ProjectRepoLinkID),
		GitHubDeliveryID:      strings.TrimSpace(spec.GitHubDeliveryID),
		GitHubInstallationID:  spec.GitHubInstallationID,
		GitHubRepoID:          spec.GitHubRepoID,
		RepoOwner:             strings.TrimSpace(spec.RepoOwner),
		RepoName:              strings.TrimSpace(spec.RepoName),
		RepoFullName:          strings.TrimSpace(spec.RepoFullName),
		TrackedBranch:         strings.TrimSpace(spec.TrackedBranch),
		CommitSHA:             strings.TrimSpace(spec.CommitSHA),
		TriggerKind:           strings.TrimSpace(spec.TriggerKind),
		PullRequestNumber:     spec.PullRequestNumber,
		PreviewEnabled:        spec.PreviewEnabled,
		ServiceTargets:        serviceTargets,
		ArtifactMetadataStage: artifactStage,
		RetryPolicy: BuildRetryPolicyRecord{
			MaxAttempts: DefaultBuildJobMaxAttempts,
			Backoff:     "linear",
		},
		CallbackExpectation: BuildCallbackExpectationRecord{
			Path:           "/api/v1/builds/callback",
			RequiredFields: buildCallbackRequiredFields(serviceTargets),
		},
	}

	workerInputJSON, err := json.Marshal(workerInput)
	if err != nil {
		return nil, err
	}
	artifactMetadataJSON, err := json.Marshal(artifactStage)
	if err != nil {
		return nil, err
	}

	job := &models.BuildJob{
		ID:                   jobID,
		ProjectID:            workerInput.ProjectID,
		ProjectRepoLinkID:    workerInput.ProjectRepoLinkID,
		GitHubDeliveryID:     workerInput.GitHubDeliveryID,
		GitHubInstallationID: workerInput.GitHubInstallationID,
		GitHubRepoID:         workerInput.GitHubRepoID,
		RepoFullName:         workerInput.RepoFullName,
		TriggerKind:          workerInput.TriggerKind,
		Status:               BuildJobStatusQueued,
		CommitSHA:            workerInput.CommitSHA,
		TrackedBranch:        workerInput.TrackedBranch,
		PullRequestNumber:    workerInput.PullRequestNumber,
		RetryCount:           0,
		MaxAttempts:          DefaultBuildJobMaxAttempts,
		WorkerInputJSON:      string(workerInputJSON),
		ArtifactMetadataJSON: string(artifactMetadataJSON),
	}
	if err := s.buildJobs.Create(job); err != nil {
		return nil, err
	}

	record, err := ToBuildJobRecord(*job)
	if err != nil {
		return nil, err
	}
	logger.Info("build_job_enqueued",
		"build_job_id", record.ID,
		"project_id", record.ProjectID,
		"commit_sha", record.CommitSHA,
		"tracked_branch", record.TrackedBranch,
		"trigger_kind", record.TriggerKind,
		"repo_full_name", record.RepoFullName,
		"service_target_count", len(record.WorkerInput.ServiceTargets),
		"service_targets", buildTargetServiceSummaries(record.WorkerInput.ServiceTargets),
		"callback_required_fields", record.WorkerInput.CallbackExpectation.RequiredFields,
	)
	return &record, nil
}

func ToBuildJobRecord(item models.BuildJob) (BuildJobRecord, error) {
	var workerInput BuildWorkerInputRecord
	if err := json.Unmarshal([]byte(item.WorkerInputJSON), &workerInput); err != nil {
		return BuildJobRecord{}, err
	}
	var artifactMetadata BuildArtifactMetadataStageRecord
	if err := json.Unmarshal([]byte(item.ArtifactMetadataJSON), &artifactMetadata); err != nil {
		return BuildJobRecord{}, err
	}

	return BuildJobRecord{
		ID:                   item.ID,
		ProjectID:            item.ProjectID,
		ProjectRepoLinkID:    item.ProjectRepoLinkID,
		GitHubDeliveryID:     item.GitHubDeliveryID,
		GitHubInstallationID: item.GitHubInstallationID,
		GitHubRepoID:         item.GitHubRepoID,
		RepoFullName:         item.RepoFullName,
		TriggerKind:          item.TriggerKind,
		Status:               item.Status,
		CommitSHA:            item.CommitSHA,
		TrackedBranch:        item.TrackedBranch,
		PullRequestNumber:    item.PullRequestNumber,
		RetryCount:           item.RetryCount,
		MaxAttempts:          item.MaxAttempts,
		WorkerInput:          workerInput,
		ArtifactMetadata:     artifactMetadata,
		StartedAt:            item.StartedAt,
		CompletedAt:          item.CompletedAt,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}, nil
}
