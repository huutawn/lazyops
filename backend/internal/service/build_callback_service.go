package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/logger"
	"lazyops-server/pkg/utils"
)

var (
	ErrBuildJobNotFound      = errors.New("build job not found")
	ErrBuildArtifactMismatch = errors.New("build artifact mismatch")
)

type UserBroadcaster interface {
	BroadcastToUser(userID string, payload any) error
}

type BuildRolloutStarter interface {
	StartDeployment(ctx context.Context, projectID, deploymentID string) (*RolloutExecutionResult, error)
}

type BuildCallbackService struct {
	projects    ProjectStore
	blueprints  BlueprintStore
	revisions   DesiredStateRevisionStore
	deployments DeploymentStore
	buildJobs   BuildJobStore
	events      UserBroadcaster
	rollouts    BuildRolloutStarter
}

func NewBuildCallbackService(
	projects ProjectStore,
	blueprints BlueprintStore,
	revisions DesiredStateRevisionStore,
	deployments DeploymentStore,
	buildJobs BuildJobStore,
	events UserBroadcaster,
) *BuildCallbackService {
	return &BuildCallbackService{
		projects:    projects,
		blueprints:  blueprints,
		revisions:   revisions,
		deployments: deployments,
		buildJobs:   buildJobs,
		events:      events,
	}
}

func (s *BuildCallbackService) WithRolloutStarter(starter BuildRolloutStarter) *BuildCallbackService {
	if s == nil {
		return s
	}
	s.rollouts = starter
	return s
}

func (s *BuildCallbackService) Handle(cmd BuildCallbackCommand) (*BuildCallbackResult, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	buildJobID := strings.TrimSpace(cmd.BuildJobID)
	commitSHA := strings.TrimSpace(cmd.CommitSHA)
	if projectID == "" || buildJobID == "" || commitSHA == "" {
		return nil, ErrInvalidInput
	}

	status, err := normalizeBuildCallbackStatus(cmd.Status)
	if err != nil {
		return nil, err
	}

	job, err := s.buildJobs.GetByIDForProject(projectID, buildJobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrBuildJobNotFound
	}
	if strings.TrimSpace(job.CommitSHA) != commitSHA {
		return nil, ErrBuildArtifactMismatch
	}

	artifactMetadata, err := normalizeBuildArtifactMetadata(
		status,
		commitSHA,
		cmd.ImageRef,
		cmd.ImageDigest,
		cmd.DetectedServices,
		cmd.DetectedFramework,
		cmd.SuggestedHealthcheck,
	)
	if err != nil {
		return nil, err
	}
	artifactMetadataJSON, err := json.Marshal(artifactMetadata)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	startedAt := job.StartedAt
	if startedAt == nil {
		startedAt = &now
	}
	completedAt := &now
	if err := s.buildJobs.UpdateResult(job.ID, status, string(artifactMetadataJSON), startedAt, completedAt, now); err != nil {
		return nil, err
	}

	job.Status = status
	job.ArtifactMetadataJSON = string(artifactMetadataJSON)
	job.StartedAt = startedAt
	job.CompletedAt = completedAt
	job.UpdatedAt = now

	buildJobRecord, err := ToBuildJobRecord(*job)
	if err != nil {
		return nil, err
	}

	result := &BuildCallbackResult{BuildJob: buildJobRecord}
	if status == BuildJobStatusSucceeded {
		revision, err := s.createArtifactReadyRevision(*job, artifactMetadata)
		if err != nil {
			return nil, err
		}
		result.Revision = revision
		deployment, err := s.createQueuedDeployment(job.ProjectID, revision)
		if err != nil {
			return nil, err
		}
		result.Deployment = deployment
		if deployment != nil {
			s.startRolloutAsync(job.ProjectID, deployment.ID, buildJobID)
		}
	}
	if status == BuildJobStatusFailed || status == BuildJobStatusCanceled {
		if err := s.broadcastFailureEvent(projectID, buildJobRecord); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *BuildCallbackService) createArtifactReadyRevision(job models.BuildJob, artifact BuildArtifactMetadataStageRecord) (*DesiredStateRevisionRecord, error) {
	if s.blueprints == nil || s.revisions == nil {
		return nil, nil
	}

	blueprint, err := s.blueprints.GetLatestByProject(job.ProjectID)
	if err != nil {
		return nil, err
	}
	if blueprint == nil {
		return nil, nil
	}

	blueprintRecord, err := ToBlueprintRecord(*blueprint)
	if err != nil {
		return nil, err
	}
	applySuggestedHealthcheckToOneClickDefaultService(&blueprintRecord, artifact)
	blueprintRecord.Compiled.ArtifactMetadata = BlueprintArtifactMetadata{
		CommitSHA:   artifact.CommitSHA,
		ArtifactRef: artifact.ArtifactRef,
		ImageRef:    artifact.ImageRef,
	}

	revisionID := utils.NewPrefixedID("rev")
	compiled := buildDesiredStateRevisionCompiledRecord(revisionID, blueprintRecord, job.TriggerKind)
	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		return nil, err
	}

	revision := &models.DesiredStateRevision{
		ID:                   revisionID,
		ProjectID:            job.ProjectID,
		BlueprintID:          blueprint.ID,
		DeploymentBindingID:  blueprintRecord.Compiled.Binding.ID,
		CommitSHA:            artifact.CommitSHA,
		TriggerKind:          job.TriggerKind,
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: string(compiledJSON),
	}
	if err := s.revisions.Create(revision); err != nil {
		return nil, err
	}

	record, err := ToDesiredStateRevisionRecord(*revision)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *BuildCallbackService) createQueuedDeployment(projectID string, revision *DesiredStateRevisionRecord) (*DeploymentRecord, error) {
	if s.deployments == nil || revision == nil {
		return nil, nil
	}
	deployment := &models.Deployment{
		ID:         utils.NewPrefixedID("dep"),
		ProjectID:  projectID,
		RevisionID: revision.ID,
		Status:     DeploymentStatusQueued,
	}
	if err := s.deployments.Create(deployment); err != nil {
		return nil, err
	}
	record := ToDeploymentRecord(*deployment)
	return &record, nil
}

func (s *BuildCallbackService) startRolloutAsync(projectID, deploymentID, buildJobID string) {
	if s.rollouts == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()

		result, err := s.rollouts.StartDeployment(ctx, projectID, deploymentID)
		if err != nil {
			logger.Warn("build_callback_rollout_start_failed",
				"project_id", projectID,
				"deployment_id", deploymentID,
				"build_job_id", buildJobID,
				"error", err.Error(),
			)
			return
		}
		logger.Info("build_callback_rollout_started",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"build_job_id", buildJobID,
			"revision_id", result.RevisionID,
			"already_started", result.AlreadyStarted,
		)
	}()
}

func (s *BuildCallbackService) broadcastFailureEvent(projectID string, buildJob BuildJobRecord) error {
	if s.events == nil || s.projects == nil {
		return nil
	}

	project, err := s.projects.GetByID(projectID)
	if err != nil {
		return err
	}
	if project == nil {
		return nil
	}

	return s.events.BroadcastToUser(project.UserID, BuildRealtimeEvent{
		Type: "build.job.failed",
		Payload: BuildFailureRealtimePayload{
			BuildJobID:       buildJob.ID,
			ProjectID:        buildJob.ProjectID,
			Status:           buildJob.Status,
			TriggerKind:      buildJob.TriggerKind,
			CommitSHA:        buildJob.CommitSHA,
			TrackedBranch:    buildJob.TrackedBranch,
			ArtifactMetadata: buildJob.ArtifactMetadata,
		},
		Meta: RealtimeMeta{
			Source: "build_callback",
			At:     time.Now().UTC(),
		},
	})
}

func normalizeBuildCallbackStatus(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case BuildJobStatusSucceeded:
		return BuildJobStatusSucceeded, nil
	case "success":
		return BuildJobStatusSucceeded, nil
	case BuildJobStatusFailed:
		return BuildJobStatusFailed, nil
	case BuildJobStatusCanceled:
		return BuildJobStatusCanceled, nil
	default:
		return "", ErrInvalidInput
	}
}

func normalizeBuildArtifactMetadata(
	status,
	commitSHA,
	imageRef,
	imageDigest string,
	detectedServices []string,
	detectedFramework string,
	suggestedHealthcheck *BuildSuggestedHealthcheckRecord,
) (BuildArtifactMetadataStageRecord, error) {
	artifact := BuildArtifactMetadataStageRecord{
		CommitSHA:            strings.TrimSpace(commitSHA),
		ImageRef:             strings.TrimSpace(imageRef),
		ImageDigest:          strings.TrimSpace(imageDigest),
		DetectedServices:     normalizeDetectedServices(detectedServices),
		DetectedFramework:    normalizeDetectedFramework(detectedFramework),
		SuggestedHealthcheck: normalizeSuggestedHealthcheck(suggestedHealthcheck),
	}
	if artifact.CommitSHA == "" {
		return BuildArtifactMetadataStageRecord{}, ErrInvalidInput
	}
	if status == BuildJobStatusSucceeded && (artifact.ImageRef == "" || artifact.ImageDigest == "") {
		return BuildArtifactMetadataStageRecord{}, ErrInvalidInput
	}
	artifact.ArtifactRef = deriveBuildArtifactRef(artifact.ImageRef, artifact.ImageDigest)
	return artifact, nil
}

func normalizeDetectedServices(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func deriveBuildArtifactRef(imageRef, imageDigest string) string {
	imageRef = strings.TrimSpace(imageRef)
	imageDigest = strings.TrimSpace(imageDigest)
	switch {
	case imageRef != "" && imageDigest != "":
		return imageRef + "@" + imageDigest
	case imageRef != "":
		return imageRef
	default:
		return ""
	}
}

func normalizeDetectedFramework(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "next":
		return "next"
	case "vite":
		return "vite"
	case "react-scripts":
		return "react-scripts"
	default:
		return ""
	}
}

func normalizeSuggestedHealthcheck(raw *BuildSuggestedHealthcheckRecord) *BuildSuggestedHealthcheckRecord {
	if raw == nil {
		return nil
	}
	port := raw.Port
	if port <= 0 {
		return nil
	}
	path := strings.TrimSpace(raw.Path)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return &BuildSuggestedHealthcheckRecord{
		Path: path,
		Port: port,
	}
}

func applySuggestedHealthcheckToOneClickDefaultService(blueprint *BlueprintRecord, artifact BuildArtifactMetadataStageRecord) {
	if blueprint == nil || artifact.SuggestedHealthcheck == nil {
		return
	}
	if !isOneClickGeneratedBlueprint(*blueprint) {
		return
	}
	if len(blueprint.Compiled.Services) != 1 {
		return
	}

	service := blueprint.Compiled.Services[0]
	if strings.TrimSpace(service.Name) != "app" || strings.TrimSpace(service.Path) != "." || !service.Public {
		return
	}
	if !isGenericFallbackHealthcheck(service.Healthcheck) {
		return
	}

	if service.Healthcheck == nil {
		service.Healthcheck = map[string]any{}
	}
	service.Healthcheck["path"] = artifact.SuggestedHealthcheck.Path
	service.Healthcheck["port"] = artifact.SuggestedHealthcheck.Port
	if _, ok := service.Healthcheck["protocol"]; !ok {
		service.Healthcheck["protocol"] = "http"
	}
	blueprint.Compiled.Services[0] = service
}

func isOneClickGeneratedBlueprint(blueprint BlueprintRecord) bool {
	return strings.HasPrefix(
		strings.TrimSpace(blueprint.Compiled.ArtifactMetadata.ArtifactRef),
		"artifact://one-click/",
	)
}

func isGenericFallbackHealthcheck(healthcheck map[string]any) bool {
	if len(healthcheck) == 0 {
		return false
	}

	port := extractHealthcheckPort(healthcheck)
	if port != 8080 {
		return false
	}

	path := strings.TrimSpace(strings.ToLower(extractHealthcheckString(healthcheck, "path")))
	if path == "" {
		path = "/"
	}
	if path != "/" {
		return false
	}

	protocol := strings.TrimSpace(strings.ToLower(extractHealthcheckString(healthcheck, "protocol")))
	return protocol == "" || protocol == "http"
}

func extractHealthcheckString(healthcheck map[string]any, key string) string {
	value, ok := healthcheck[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func extractHealthcheckPort(healthcheck map[string]any) int {
	value, ok := healthcheck["port"]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
