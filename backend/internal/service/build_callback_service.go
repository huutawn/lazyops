package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
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
	compiler    *ServiceInventoryBlueprintCompiler
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

func (s *BuildCallbackService) WithServiceInventoryCompiler(compiler *ServiceInventoryBlueprintCompiler) *BuildCallbackService {
	if s == nil {
		return s
	}
	s.compiler = compiler
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
		cmd.DetectedPorts,
		cmd.PortDetectionSource,
		cmd.PortDetectionConfidence,
		cmd.SuggestedTargetPort,
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
		revision, appliedServices, err := s.createArtifactReadyRevision(*job, artifactMetadata)
		if err != nil {
			return nil, err
		}
		if len(appliedServices) > 0 {
			artifactMetadata.AppliedServices = normalizeDetectedServices(appliedServices)
			artifactMetadataJSON, err = json.Marshal(artifactMetadata)
			if err != nil {
				return nil, err
			}
			if err := s.buildJobs.UpdateResult(job.ID, status, string(artifactMetadataJSON), startedAt, completedAt, now); err != nil {
				return nil, err
			}
			job.ArtifactMetadataJSON = string(artifactMetadataJSON)
			buildJobRecord, err = ToBuildJobRecord(*job)
			if err != nil {
				return nil, err
			}
			result.BuildJob = buildJobRecord
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

func (s *BuildCallbackService) createArtifactReadyRevision(job models.BuildJob, artifact BuildArtifactMetadataStageRecord) (*DesiredStateRevisionRecord, []string, error) {
	if s.revisions == nil {
		return nil, nil, nil
	}

	var blueprintRecord BlueprintRecord
	appliedServices := []string{}
	if s.compiler != nil {
		project, err := s.projects.GetByID(job.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		if project == nil {
			return nil, nil, ErrProjectNotFound
		}
		compiled, err := s.compiler.Compile(*project, ServiceInventoryBlueprintCompileInput{
			TriggerKind: job.TriggerKind,
			Artifact: BlueprintArtifactMetadata{
				CommitSHA:   artifact.CommitSHA,
				ArtifactRef: artifact.ArtifactRef,
				ImageRef:    artifact.ImageRef,
			},
		})
		if err != nil {
			return nil, nil, err
		}
		blueprintRecord = compiled.Blueprint
		appliedServices = compiled.AppliedServices
	} else {
		if s.blueprints == nil {
			return nil, nil, nil
		}
		blueprint, err := s.blueprints.GetLatestByProject(job.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		if blueprint == nil {
			return nil, nil, nil
		}
		parsed, err := ToBlueprintRecord(*blueprint)
		if err != nil {
			return nil, nil, err
		}
		blueprintRecord = parsed
		appliedServices = applyArtifactToBlueprintServices(&blueprintRecord, artifact)
	}

	blueprintRecord.Compiled.ArtifactMetadata = BlueprintArtifactMetadata{
		CommitSHA:   artifact.CommitSHA,
		ArtifactRef: artifact.ArtifactRef,
		ImageRef:    artifact.ImageRef,
	}

	revisionID := utils.NewPrefixedID("rev")
	compiled := buildDesiredStateRevisionCompiledRecord(revisionID, blueprintRecord, job.TriggerKind)
	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		return nil, nil, err
	}

	revision := &models.DesiredStateRevision{
		ID:                   revisionID,
		ProjectID:            job.ProjectID,
		BlueprintID:          blueprintRecord.ID,
		DeploymentBindingID:  blueprintRecord.Compiled.Binding.ID,
		Namespace:            blueprintRecord.Compiled.Namespace,
		CommitSHA:            artifact.CommitSHA,
		TriggerKind:          job.TriggerKind,
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: string(compiledJSON),
	}
	if err := s.revisions.Create(revision); err != nil {
		return nil, nil, err
	}

	record, err := ToDesiredStateRevisionRecord(*revision)
	if err != nil {
		return nil, nil, err
	}
	return &record, appliedServices, nil
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
	detectedPorts []ServiceDetectedPortRecord,
	portDetectionSource string,
	portDetectionConfidence string,
	suggestedTargetPort int,
	detectedFramework string,
	suggestedHealthcheck *BuildSuggestedHealthcheckRecord,
) (BuildArtifactMetadataStageRecord, error) {
	artifact := BuildArtifactMetadataStageRecord{
		CommitSHA:               strings.TrimSpace(commitSHA),
		ImageRef:                strings.TrimSpace(imageRef),
		ImageDigest:             strings.TrimSpace(imageDigest),
		DetectedServices:        normalizeDetectedServices(detectedServices),
		DetectedPorts:           normalizeDetectedPorts(detectedPorts),
		PortDetectionSource:     normalizePortDetectionSource(portDetectionSource),
		PortDetectionConfidence: normalizePortDetectionConfidence(portDetectionConfidence),
		SuggestedTargetPort:     normalizeSuggestedTargetPort(suggestedTargetPort, detectedPorts, suggestedHealthcheck),
		DetectedFramework:       normalizeDetectedFramework(detectedFramework),
		SuggestedHealthcheck:    normalizeSuggestedHealthcheck(suggestedHealthcheck),
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

func normalizeDetectedPorts(items []ServiceDetectedPortRecord) []ServiceDetectedPortRecord {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]ServiceDetectedPortRecord, 0, len(items))
	for _, item := range items {
		if item.Port <= 0 {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		key := fmt.Sprintf("%d/%s", item.Port, protocol)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ServiceDetectedPortRecord{
			Port:     item.Port,
			Protocol: protocol,
			Name:     strings.TrimSpace(item.Name),
			Exposed:  item.Exposed,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port == out[j].Port {
			return out[i].Protocol < out[j].Protocol
		}
		return out[i].Port < out[j].Port
	})
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

func normalizePortDetectionSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "docker_inspect":
		return "docker_inspect"
	case "registry_api":
		return "registry_api"
	default:
		return ""
	}
}

func normalizePortDetectionConfidence(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		return ""
	}
}

func normalizeSuggestedTargetPort(port int, detectedPorts []ServiceDetectedPortRecord, suggestedHealthcheck *BuildSuggestedHealthcheckRecord) int {
	if port > 0 {
		return port
	}
	if suggestedHealthcheck != nil && suggestedHealthcheck.Port > 0 {
		return suggestedHealthcheck.Port
	}
	normalized := normalizeDetectedPorts(detectedPorts)
	if len(normalized) == 1 {
		return normalized[0].Port
	}
	for _, item := range normalized {
		if item.Port >= 1024 {
			return item.Port
		}
	}
	if len(normalized) > 0 {
		return normalized[0].Port
	}
	return 0
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

func applyArtifactToBlueprintServices(blueprint *BlueprintRecord, artifact BuildArtifactMetadataStageRecord) []string {
	if blueprint == nil || len(blueprint.Compiled.Services) == 0 {
		return nil
	}
	targetIndexes := resolveArtifactTargetServiceIndexes(*blueprint, artifact)
	if len(targetIndexes) == 0 {
		return nil
	}

	applied := make([]string, 0, len(targetIndexes))
	for _, index := range targetIndexes {
		if index < 0 || index >= len(blueprint.Compiled.Services) {
			continue
		}
		service := blueprint.Compiled.Services[index]
		forceFallbackOverride := isOneClickGeneratedBlueprint(*blueprint) &&
			strings.TrimSpace(service.Name) == "app" &&
			strings.TrimSpace(service.Path) == "." &&
			service.Public &&
			isGenericFallbackHealthcheck(service.Healthcheck)
		applyArtifactToBlueprintService(&service, artifact, forceFallbackOverride)
		blueprint.Compiled.Services[index] = service
		applied = append(applied, service.Name)
	}
	return applied
}

func applySuggestedHealthcheckToOneClickDefaultService(blueprint *BlueprintRecord, artifact BuildArtifactMetadataStageRecord) {
	applyArtifactToBlueprintServices(blueprint, artifact)
}

func resolveArtifactTargetServiceIndexes(blueprint BlueprintRecord, artifact BuildArtifactMetadataStageRecord) []int {
	services := blueprint.Compiled.Services
	if len(services) == 0 {
		return nil
	}
	if len(services) == 1 {
		return []int{0}
	}

	if matched := matchArtifactDetectedServices(services, artifact.DetectedServices); len(matched) == 1 {
		return matched
	}

	if index := resolveFrameworkPreferredServiceIndex(services, artifact); index >= 0 {
		return []int{index}
	}

	if index := resolveSingleAppLikeServiceIndex(services); index >= 0 {
		return []int{index}
	}

	return nil
}

func matchArtifactDetectedServices(services []BlueprintServiceContractRecord, detectedServices []string) []int {
	if len(services) == 0 || len(detectedServices) == 0 {
		return nil
	}

	nameIndex := make(map[string]int, len(services))
	pathIndex := make(map[string]int, len(services))
	for index, service := range services {
		name := normalizeArtifactServiceMatcher(service.Name)
		if name != "" {
			nameIndex[name] = index
		}
		for _, candidate := range buildArtifactPathMatchers(service.Path) {
			if candidate != "" {
				pathIndex[candidate] = index
			}
		}
	}

	seen := make(map[int]struct{})
	matches := make([]int, 0, len(detectedServices))
	for _, raw := range detectedServices {
		candidate := normalizeArtifactServiceMatcher(raw)
		if candidate == "" {
			continue
		}
		index, ok := nameIndex[candidate]
		if !ok {
			index, ok = pathIndex[candidate]
		}
		if !ok {
			continue
		}
		if _, exists := seen[index]; exists {
			continue
		}
		seen[index] = struct{}{}
		matches = append(matches, index)
	}

	sort.Ints(matches)
	return matches
}

func resolveFrameworkPreferredServiceIndex(services []BlueprintServiceContractRecord, artifact BuildArtifactMetadataStageRecord) int {
	switch strings.TrimSpace(artifact.DetectedFramework) {
	case "next", "vite", "react-scripts":
	default:
		return -1
	}

	indexes := make([]int, 0, len(services))
	for index, service := range services {
		if service.Public || strings.TrimSpace(service.RuntimeProfile) == "web" {
			indexes = append(indexes, index)
		}
	}
	if len(indexes) == 1 {
		return indexes[0]
	}
	return -1
}

func resolveSingleAppLikeServiceIndex(services []BlueprintServiceContractRecord) int {
	indexes := make([]int, 0, len(services))
	for index, service := range services {
		if isManagedInternalServiceKind(service.Kind) {
			continue
		}
		indexes = append(indexes, index)
	}
	if len(indexes) == 1 {
		return indexes[0]
	}
	return -1
}

func isManagedInternalServiceKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "postgres", "mysql", "redis", "rabbitmq":
		return true
	default:
		return false
	}
}

func buildArtifactPathMatchers(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	candidates := []string{
		path,
		strings.TrimPrefix(path, "./"),
		filepath.Base(path),
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		normalized := normalizeArtifactServiceMatcher(candidate)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeArtifactServiceMatcher(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "./")
	value = strings.Trim(value, "/")
	return value
}

func applyArtifactToBlueprintService(service *BlueprintServiceContractRecord, artifact BuildArtifactMetadataStageRecord, forceFallbackOverride bool) {
	if service == nil {
		return
	}
	service.ImageRef = artifact.ImageRef
	service.ImageDigest = artifact.ImageDigest
	service.DetectedPorts = artifact.DetectedPorts

	if artifact.SuggestedTargetPort > 0 {
		if forceFallbackOverride || service.TargetPort <= 0 {
			service.TargetPort = artifact.SuggestedTargetPort
		}
		if forceFallbackOverride || service.ServicePort <= 0 {
			service.ServicePort = artifact.SuggestedTargetPort
		}
	}

	if artifact.SuggestedHealthcheck == nil {
		return
	}
	if service.Healthcheck == nil {
		service.Healthcheck = map[string]any{}
	}
	if forceFallbackOverride || len(service.Healthcheck) == 0 {
		service.Healthcheck["path"] = artifact.SuggestedHealthcheck.Path
		service.Healthcheck["port"] = artifact.SuggestedHealthcheck.Port
		service.Healthcheck["protocol"] = "http"
	}
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
