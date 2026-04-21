package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/logger"
	"lazyops-server/pkg/utils"
)

var (
	ErrBuildJobNotFound        = errors.New("build job not found")
	ErrBuildArtifactMismatch   = errors.New("build artifact mismatch")
	ErrPortResolutionFailed    = errors.New("port resolution failed")
	ErrPortResolutionAmbiguous = errors.New("port resolution ambiguous")
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
	if projectID == "" || buildJobID == "" {
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
	commitSHA := strings.TrimSpace(cmd.CommitSHA)
	storedCommitSHA := strings.TrimSpace(job.CommitSHA)
	if commitSHA == "" {
		commitSHA = storedCommitSHA
	}
	if commitSHA == "" {
		return nil, ErrInvalidInput
	}
	if storedCommitSHA != "" && storedCommitSHA != commitSHA {
		return nil, ErrBuildArtifactMismatch
	}
	buildJobRecord, err := ToBuildJobRecord(*job)
	if err != nil {
		return nil, err
	}

	artifactMetadata, err := normalizeBuildArtifactMetadata(
		status,
		commitSHA,
		cmd.ImageRef,
		cmd.ImageDigest,
		cmd.ServiceArtifacts,
		cmd.DetectedServices,
		cmd.DetectedPorts,
		cmd.PortDetectionSource,
		cmd.PortDetectionConfidence,
		cmd.SuggestedTargetPort,
		cmd.DetectedFramework,
		cmd.SuggestedHealthcheck,
		cmd.PortResolutionStatus,
		cmd.PortResolutionSource,
		cmd.PortResolutionReason,
		cmd.CandidatePorts,
	)
	if err != nil {
		return nil, err
	}
	if err := validateBuildCallbackArtifactMetadata(buildJobRecord.WorkerInput.ServiceTargets, status, artifactMetadata); err != nil {
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
	if err := s.buildJobs.UpdateResult(job.ID, status, commitSHA, string(artifactMetadataJSON), startedAt, completedAt, now); err != nil {
		return nil, err
	}

	job.Status = status
	job.CommitSHA = commitSHA
	job.ArtifactMetadataJSON = string(artifactMetadataJSON)
	job.StartedAt = startedAt
	job.CompletedAt = completedAt
	job.UpdatedAt = now

	buildJobRecord, err = ToBuildJobRecord(*job)
	if err != nil {
		return nil, err
	}

	result := &BuildCallbackResult{BuildJob: buildJobRecord}
	if status == BuildJobStatusSucceeded {
		revision, appliedServices, err := s.createArtifactReadyRevision(*job, buildJobRecord.WorkerInput.ServiceTargets, artifactMetadata)
		if err != nil {
			return nil, err
		}
		if len(appliedServices) > 0 {
			artifactMetadata.AppliedServices = normalizeDetectedServices(appliedServices)
			artifactMetadataJSON, err = json.Marshal(artifactMetadata)
			if err != nil {
				return nil, err
			}
			if err := s.buildJobs.UpdateResult(job.ID, status, commitSHA, string(artifactMetadataJSON), startedAt, completedAt, now); err != nil {
				return nil, err
			}
			job.CommitSHA = commitSHA
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

func (s *BuildCallbackService) createArtifactReadyRevision(job models.BuildJob, expectedTargets []BuildTargetServiceRecord, artifact BuildArtifactMetadataStageRecord) (*DesiredStateRevisionRecord, []string, error) {
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
				CommitSHA:        artifact.CommitSHA,
				ArtifactRef:      artifact.ArtifactRef,
				ImageRef:         artifact.ImageRef,
				ServiceArtifacts: cloneBuildServiceArtifacts(artifact.ServiceArtifacts),
			},
		})
		if err != nil {
			return nil, nil, err
		}
		blueprintRecord = compiled.Blueprint
		appliedServices = compiled.AppliedServices
		if additionalApplied := applyArtifactToBlueprintServices(&blueprintRecord, artifact); len(additionalApplied) > 0 {
			appliedServices = normalizeDetectedServices(append(appliedServices, additionalApplied...))
		}
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
	if err := validateResolvedBuildArtifacts(expectedTargets, blueprintRecord.Compiled.Services, artifact); err != nil {
		return nil, nil, err
	}

	blueprintRecord.Compiled.ArtifactMetadata = BlueprintArtifactMetadata{
		CommitSHA:        artifact.CommitSHA,
		ArtifactRef:      artifact.ArtifactRef,
		ImageRef:         artifact.ImageRef,
		ServiceArtifacts: cloneBuildServiceArtifacts(artifact.ServiceArtifacts),
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
	serviceArtifacts []BuildServiceArtifactRecord,
	detectedServices []string,
	detectedPorts []ServiceDetectedPortRecord,
	portDetectionSource string,
	portDetectionConfidence string,
	suggestedTargetPort int,
	detectedFramework string,
	suggestedHealthcheck *BuildSuggestedHealthcheckRecord,
	portResolutionStatus string,
	portResolutionSource string,
	portResolutionReason string,
	candidatePorts []int,
) (BuildArtifactMetadataStageRecord, error) {
	artifact := BuildArtifactMetadataStageRecord{
		CommitSHA:               strings.TrimSpace(commitSHA),
		ImageRef:                strings.TrimSpace(imageRef),
		ImageDigest:             strings.TrimSpace(imageDigest),
		ServiceArtifacts:        normalizeBuildServiceArtifacts(serviceArtifacts),
		DetectedServices:        normalizeDetectedServices(detectedServices),
		DetectedPorts:           normalizeDetectedPorts(detectedPorts),
		PortDetectionSource:     normalizePortDetectionSource(portDetectionSource),
		PortDetectionConfidence: normalizePortDetectionConfidence(portDetectionConfidence),
		SuggestedTargetPort:     normalizeSuggestedTargetPort(suggestedTargetPort, detectedPorts, suggestedHealthcheck, candidatePorts, portResolutionStatus),
		DetectedFramework:       normalizeDetectedFramework(detectedFramework),
		SuggestedHealthcheck:    normalizeSuggestedHealthcheck(suggestedHealthcheck),
		PortResolutionStatus:    normalizePortResolutionStatus(portResolutionStatus),
		PortResolutionSource:    normalizePortResolutionSource(portResolutionSource),
		PortResolutionReason:    strings.TrimSpace(portResolutionReason),
		CandidatePorts:          normalizeCandidatePorts(candidatePorts),
	}
	if artifact.CommitSHA == "" {
		return BuildArtifactMetadataStageRecord{}, ErrInvalidInput
	}
	if len(artifact.DetectedServices) == 0 && len(artifact.ServiceArtifacts) > 0 {
		detected := make([]string, 0, len(artifact.ServiceArtifacts))
		for _, item := range artifact.ServiceArtifacts {
			detected = append(detected, item.ServiceName)
		}
		artifact.DetectedServices = normalizeDetectedServices(detected)
	}
	if status == BuildJobStatusSucceeded &&
		(artifact.ImageRef == "" || artifact.ImageDigest == "") &&
		len(artifact.ServiceArtifacts) == 0 {
		return BuildArtifactMetadataStageRecord{}, ErrInvalidInput
	}
	artifact.ArtifactRef = deriveBuildArtifactRef(artifact.ImageRef, artifact.ImageDigest)
	return artifact, nil
}

func validateBuildCallbackArtifactMetadata(expectedTargets []BuildTargetServiceRecord, status string, artifact BuildArtifactMetadataStageRecord) error {
	if status != BuildJobStatusSucceeded {
		return nil
	}
	normalizedTargets := normalizeBuildTargetServices(expectedTargets)
	if len(normalizedTargets) == 0 {
		return nil
	}
	if err := validateStagedServiceArtifactCoverage(normalizedTargets, artifact.ServiceArtifacts); err != nil {
		return err
	}
	if err := validateServiceArtifactImageReferences(artifact.ServiceArtifacts); err != nil {
		return err
	}
	return nil
}

func validateStagedServiceArtifactCoverage(expectedTargets []BuildTargetServiceRecord, artifacts []BuildServiceArtifactRecord) error {
	if len(expectedTargets) == 0 {
		return nil
	}
	if len(artifacts) == 0 {
		if len(expectedTargets) > 1 {
			return newBuildArtifactMismatch(
				fmt.Sprintf(
					"multi-service build callback requires metadata.service_artifacts for staged targets %s",
					strings.Join(buildTargetServiceSummaries(expectedTargets), ", "),
				),
			)
		}
		return nil
	}

	expectedByKey := make(map[string]BuildTargetServiceRecord, len(expectedTargets))
	for _, target := range expectedTargets {
		expectedByKey[buildTargetServiceKey(target.ServiceName, target.ServicePath)] = target
	}

	missing := make([]string, 0)
	unexpected := make([]string, 0)
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		key := buildTargetServiceKey(artifact.ServiceName, artifact.ServicePath)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if _, ok := expectedByKey[key]; !ok {
			unexpected = append(unexpected, strings.TrimSpace(artifact.ServiceName)+"@"+strings.TrimSpace(artifact.ServicePath))
		}
	}
	for key, target := range expectedByKey {
		if _, ok := seen[key]; ok {
			continue
		}
		missing = append(missing, strings.TrimSpace(target.ServiceName)+"@"+strings.TrimSpace(target.ServicePath))
	}
	sort.Strings(missing)
	sort.Strings(unexpected)

	if len(missing) == 0 && len(unexpected) == 0 {
		return nil
	}

	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, "missing service_artifacts for "+strings.Join(missing, ", "))
	}
	if len(unexpected) > 0 {
		parts = append(parts, "unexpected service_artifacts for "+strings.Join(unexpected, ", "))
	}
	return newBuildArtifactMismatch(strings.Join(parts, "; "))
}

func validateResolvedBuildArtifacts(expectedTargets []BuildTargetServiceRecord, services []BlueprintServiceContractRecord, artifact BuildArtifactMetadataStageRecord) error {
	if err := validateServiceArtifactImageReferences(artifact.ServiceArtifacts); err != nil {
		return err
	}
	if err := validateServiceArtifactsAgainstBlueprintServices(services, artifact.ServiceArtifacts); err != nil {
		return err
	}
	if len(normalizeBuildTargetServices(expectedTargets)) > 1 && len(artifact.ServiceArtifacts) == 0 {
		return newBuildArtifactMismatch(
			"multi-service build callback cannot fall back to a shared top-level image without metadata.service_artifacts",
		)
	}
	if err := validateResolvedServicePorts(expectedTargets, services, artifact); err != nil {
		return err
	}
	return nil
}

type portResolutionSnapshot struct {
	Status         string
	Source         string
	Reason         string
	CandidatePorts []int
}

func validateResolvedServicePorts(expectedTargets []BuildTargetServiceRecord, services []BlueprintServiceContractRecord, artifact BuildArtifactMetadataStageRecord) error {
	if len(services) == 0 {
		return nil
	}

	expectedByKey := make(map[string]BuildTargetServiceRecord, len(expectedTargets))
	for _, target := range normalizeBuildTargetServices(expectedTargets) {
		expectedByKey[buildTargetServiceKey(target.ServiceName, target.ServicePath)] = target
	}
	resolutionByIndex := buildArtifactResolutionSnapshots(services, artifact)

	for index, service := range services {
		expectedTarget, hasExpectedTarget := expectedByKey[buildTargetServiceKey(service.Name, service.Path)]
		if !hasExpectedTarget && strings.TrimSpace(service.ImageRef) == "" {
			continue
		}
		if !requiresResolvedServicePort(service, expectedTarget) {
			if err := validateServiceHealthcheckConsistency(service); err != nil {
				return err
			}
			continue
		}

		snapshot, ok := resolutionByIndex[index]
		resolvedPort := firstPositive(service.TargetPort, service.ServicePort)
		if resolvedPort <= 0 {
			if !ok {
				snapshot = portResolutionSnapshot{
					Status:         BuildPortResolutionStatusUnresolved,
					Reason:         "no port resolution metadata was attached to the build artifact",
					CandidatePorts: nil,
				}
			}
			return newPortResolutionError(service, snapshot, "service requires a resolved target_port/service_port before rollout")
		}
		if hasExpectedTarget && !hasDeclaredBuildTargetPort(expectedTarget) && !isAcceptedResolvedServicePortSource(snapshot.Source) {
			if strings.TrimSpace(snapshot.Reason) == "" {
				snapshot.Reason = "distributed-k3s only accepts ports resolved from explicit config or image EXPOSE"
			}
			return newPortResolutionError(
				service,
				snapshot,
				"service requires a resolved target_port/service_port from explicit config or a single TCP EXPOSE before rollout",
			)
		}
		if err := validateServiceHealthcheckConsistency(service); err != nil {
			return err
		}
	}
	return nil
}

func buildArtifactResolutionSnapshots(services []BlueprintServiceContractRecord, artifact BuildArtifactMetadataStageRecord) map[int]portResolutionSnapshot {
	if len(services) == 0 {
		return nil
	}
	out := make(map[int]portResolutionSnapshot, len(services))
	if len(artifact.ServiceArtifacts) > 0 {
		for _, item := range artifact.ServiceArtifacts {
			index := resolveServiceArtifactTargetIndex(services, item)
			if index < 0 {
				continue
			}
			out[index] = portResolutionSnapshot{
				Status:         normalizePortResolutionStatus(item.PortResolutionStatus),
				Source:         normalizePortResolutionSource(item.PortResolutionSource),
				Reason:         strings.TrimSpace(item.PortResolutionReason),
				CandidatePorts: normalizeCandidatePorts(item.CandidatePorts),
			}
		}
		return out
	}

	indexes := resolveArtifactTargetServiceIndexes(
		BlueprintRecord{Compiled: BlueprintCompiledContractRecord{Services: services}},
		artifact,
	)
	for _, index := range indexes {
		if index < 0 || index >= len(services) {
			continue
		}
		out[index] = portResolutionSnapshot{
			Status:         normalizePortResolutionStatus(artifact.PortResolutionStatus),
			Source:         normalizePortResolutionSource(artifact.PortResolutionSource),
			Reason:         strings.TrimSpace(artifact.PortResolutionReason),
			CandidatePorts: normalizeCandidatePorts(artifact.CandidatePorts),
		}
	}
	return out
}

func requiresResolvedServicePort(service BlueprintServiceContractRecord, expectedTarget BuildTargetServiceRecord) bool {
	if isManagedInternalServiceKind(service.Kind) {
		return false
	}
	if declaredServicePort(expectedTarget, service) > 0 {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(service.RuntimeProfile), "worker") && !service.Public && extractHealthcheckPort(service.Healthcheck) <= 0 {
		return false
	}
	if strings.TrimSpace(service.ImageRef) == "" && strings.TrimSpace(service.SourceType) != serviceSourceTypeRepo && strings.TrimSpace(expectedTarget.ServiceName) == "" {
		return false
	}
	return true
}

func declaredServicePort(expectedTarget BuildTargetServiceRecord, service BlueprintServiceContractRecord) int {
	if expectedTarget.DeclaredTargetPort > 0 {
		return expectedTarget.DeclaredTargetPort
	}
	if expectedTarget.DeclaredServicePort > 0 {
		return expectedTarget.DeclaredServicePort
	}
	if port := extractHealthcheckPort(expectedTarget.DeclaredHealthcheck); port > 0 {
		return port
	}
	return firstPositive(service.TargetPort, service.ServicePort)
}

func hasDeclaredBuildTargetPort(expectedTarget BuildTargetServiceRecord) bool {
	return expectedTarget.DeclaredTargetPort > 0 ||
		expectedTarget.DeclaredServicePort > 0 ||
		extractHealthcheckPort(expectedTarget.DeclaredHealthcheck) > 0
}

func isAcceptedResolvedServicePortSource(raw string) bool {
	switch normalizePortResolutionSource(raw) {
	case BuildPortResolutionSourceExplicit, BuildPortResolutionSourceDockerInspect:
		return true
	default:
		return false
	}
}

func validateServiceHealthcheckConsistency(service BlueprintServiceContractRecord) error {
	if len(service.Healthcheck) == 0 {
		return nil
	}
	healthPort := extractHealthcheckPort(service.Healthcheck)
	healthPath := strings.TrimSpace(extractHealthcheckString(service.Healthcheck, "path"))
	if healthPath != "" && healthPort <= 0 {
		return fmt.Errorf("%w: service %q defines healthcheck.path %q without healthcheck.port", ErrPortResolutionFailed, service.Name, healthPath)
	}
	resolvedPort := firstPositive(service.TargetPort, service.ServicePort)
	if healthPort > 0 && resolvedPort > 0 && healthPort != resolvedPort {
		return fmt.Errorf(
			"%w: service %q resolved target port %d conflicts with healthcheck.port %d; update the service port or healthcheck to match",
			ErrPortResolutionFailed,
			service.Name,
			resolvedPort,
			healthPort,
		)
	}
	return nil
}

func newPortResolutionError(service BlueprintServiceContractRecord, snapshot portResolutionSnapshot, summary string) error {
	base := ErrPortResolutionFailed
	status := normalizePortResolutionStatus(snapshot.Status)
	if status == "" && len(snapshot.CandidatePorts) > 1 {
		status = BuildPortResolutionStatusAmbiguous
	}
	if status == BuildPortResolutionStatusAmbiguous {
		base = ErrPortResolutionAmbiguous
	}
	parts := []string{
		fmt.Sprintf("service %q: %s", strings.TrimSpace(service.Name), strings.TrimSpace(summary)),
	}
	if status != "" {
		parts = append(parts, "status="+status)
	}
	if source := normalizePortResolutionSource(snapshot.Source); source != "" {
		parts = append(parts, "source="+source)
	}
	if len(snapshot.CandidatePorts) > 0 {
		ports := make([]string, 0, len(snapshot.CandidatePorts))
		for _, port := range snapshot.CandidatePorts {
			ports = append(ports, fmt.Sprintf("%d", port))
		}
		parts = append(parts, "candidate_ports=["+strings.Join(ports, ",")+"]")
	}
	if reason := strings.TrimSpace(snapshot.Reason); reason != "" {
		parts = append(parts, reason)
	}
	parts = append(parts, "declare target_port/service_port or a precise healthcheck.port to unblock the rollout")
	return fmt.Errorf("%w: %s", base, strings.Join(parts, "; "))
}

func validateServiceArtifactsAgainstBlueprintServices(services []BlueprintServiceContractRecord, artifacts []BuildServiceArtifactRecord) error {
	if len(artifacts) == 0 {
		return nil
	}
	ambiguous := make([]string, 0)
	duplicates := make([]string, 0)
	seen := make(map[int]string, len(artifacts))
	unmapped := make([]string, 0)
	for _, artifact := range artifacts {
		matches := resolveServiceArtifactTargetIndexes(services, artifact)
		summary := strings.TrimSpace(artifact.ServiceName) + "@" + strings.TrimSpace(artifact.ServicePath)
		switch len(matches) {
		case 0:
			unmapped = append(unmapped, summary)
			continue
		case 1:
			if prior, exists := seen[matches[0]]; exists {
				duplicates = append(duplicates, prior+" and "+summary)
				continue
			}
			seen[matches[0]] = summary
		default:
			ambiguous = append(ambiguous, summary)
		}
	}
	sort.Strings(ambiguous)
	sort.Strings(unmapped)
	sort.Strings(duplicates)
	if len(unmapped) == 0 && len(duplicates) == 0 && len(ambiguous) == 0 {
		return nil
	}
	parts := make([]string, 0, 3)
	if len(unmapped) > 0 {
		parts = append(parts, "service_artifacts do not map to compiled services: "+strings.Join(unmapped, ", "))
	}
	if len(ambiguous) > 0 {
		parts = append(parts, "service_artifacts map ambiguously to multiple compiled services: "+strings.Join(ambiguous, ", "))
	}
	if len(duplicates) > 0 {
		parts = append(parts, "service_artifacts map ambiguously to the same compiled service: "+strings.Join(duplicates, ", "))
	}
	return newBuildArtifactMismatch(strings.Join(parts, "; "))
}

func validateServiceArtifactImageReferences(artifacts []BuildServiceArtifactRecord) error {
	if len(artifacts) < 2 {
		return nil
	}
	digestByImageRef := make(map[string]string, len(artifacts))
	conflicts := make([]string, 0)
	for _, artifact := range artifacts {
		imageRef := strings.TrimSpace(artifact.ImageRef)
		imageDigest := strings.TrimSpace(artifact.ImageDigest)
		if imageRef == "" || imageDigest == "" {
			continue
		}
		if priorDigest, exists := digestByImageRef[imageRef]; exists {
			if priorDigest != imageDigest {
				conflicts = append(conflicts, imageRef)
			}
			continue
		}
		digestByImageRef[imageRef] = imageDigest
	}
	if len(conflicts) == 0 {
		return nil
	}
	sort.Strings(conflicts)
	conflicts = slices.Compact(conflicts)
	return newBuildArtifactMismatch(
		"service_artifacts reuse the same image_ref with different image_digest values: " + strings.Join(conflicts, ", "),
	)
}

func newBuildArtifactMismatch(message string) error {
	return fmt.Errorf("%w: %s", ErrBuildArtifactMismatch, strings.TrimSpace(message))
}

func normalizeBuildServiceArtifacts(items []BuildServiceArtifactRecord) []BuildServiceArtifactRecord {
	if len(items) == 0 {
		return nil
	}
	out := make([]BuildServiceArtifactRecord, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ServiceName)
		path := strings.TrimSpace(item.ServicePath)
		imageRef := strings.TrimSpace(item.ImageRef)
		imageDigest := strings.TrimSpace(item.ImageDigest)
		if name == "" || path == "" || imageRef == "" || imageDigest == "" {
			continue
		}
		key := normalizeArtifactServiceMatcher(name) + "|" + normalizeArtifactServiceMatcher(path)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		artifact := BuildServiceArtifactRecord{
			ServiceName:             name,
			ServicePath:             path,
			ImageRef:                imageRef,
			ImageDigest:             imageDigest,
			DetectedPorts:           normalizeDetectedPorts(item.DetectedPorts),
			PortDetectionSource:     normalizePortDetectionSource(item.PortDetectionSource),
			PortDetectionConfidence: normalizePortDetectionConfidence(item.PortDetectionConfidence),
			SuggestedTargetPort:     normalizeSuggestedTargetPort(item.SuggestedTargetPort, item.DetectedPorts, item.SuggestedHealthcheck, item.CandidatePorts, item.PortResolutionStatus),
			DetectedFramework:       normalizeDetectedFramework(item.DetectedFramework),
			SuggestedHealthcheck:    normalizeSuggestedHealthcheck(item.SuggestedHealthcheck),
			PortResolutionStatus:    normalizePortResolutionStatus(item.PortResolutionStatus),
			PortResolutionSource:    normalizePortResolutionSource(item.PortResolutionSource),
			PortResolutionReason:    strings.TrimSpace(item.PortResolutionReason),
			CandidatePorts:          normalizeCandidatePorts(item.CandidatePorts),
		}
		artifact.ArtifactRef = deriveBuildArtifactRef(artifact.ImageRef, artifact.ImageDigest)
		out = append(out, artifact)
	}
	return out
}

func cloneBuildServiceArtifacts(items []BuildServiceArtifactRecord) []BuildServiceArtifactRecord {
	if len(items) == 0 {
		return nil
	}
	out := make([]BuildServiceArtifactRecord, 0, len(items))
	for _, item := range items {
		cloned := item
		cloned.DetectedPorts = cloneDetectedPorts(item.DetectedPorts)
		cloned.SuggestedHealthcheck = cloneSuggestedHealthcheck(item.SuggestedHealthcheck)
		cloned.CandidatePorts = cloneIntSlice(item.CandidatePorts)
		out = append(out, cloned)
	}
	return out
}

func cloneSuggestedHealthcheck(item *BuildSuggestedHealthcheckRecord) *BuildSuggestedHealthcheckRecord {
	if item == nil {
		return nil
	}
	cloned := *item
	return &cloned
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
	case "smoke_run":
		return "smoke_run"
	case "mixed":
		return "mixed"
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

func normalizePortResolutionStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case BuildPortResolutionStatusResolved:
		return BuildPortResolutionStatusResolved
	case BuildPortResolutionStatusAmbiguous:
		return BuildPortResolutionStatusAmbiguous
	case BuildPortResolutionStatusUnresolved:
		return BuildPortResolutionStatusUnresolved
	default:
		return ""
	}
}

func normalizePortResolutionSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case BuildPortResolutionSourceExplicit:
		return BuildPortResolutionSourceExplicit
	case BuildPortResolutionSourceDockerInspect:
		return BuildPortResolutionSourceDockerInspect
	case BuildPortResolutionSourceFrameworkHint:
		return BuildPortResolutionSourceFrameworkHint
	case BuildPortResolutionSourceSmokeRun:
		return BuildPortResolutionSourceSmokeRun
	case BuildPortResolutionSourceStartHint:
		return BuildPortResolutionSourceStartHint
	case BuildPortResolutionSourceMixed:
		return BuildPortResolutionSourceMixed
	case BuildPortResolutionSourceInternal:
		return BuildPortResolutionSourceInternal
	default:
		return ""
	}
}

func normalizeSuggestedTargetPort(port int, detectedPorts []ServiceDetectedPortRecord, suggestedHealthcheck *BuildSuggestedHealthcheckRecord, candidatePorts []int, portResolutionStatus string) int {
	if port > 0 {
		return port
	}
	if normalizePortResolutionStatus(portResolutionStatus) == BuildPortResolutionStatusResolved {
		normalizedCandidates := normalizeCandidatePorts(candidatePorts)
		if len(normalizedCandidates) == 1 {
			return normalizedCandidates[0]
		}
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

func normalizeCandidatePorts(items []int) []int {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(items))
	out := make([]int, 0, len(items))
	for _, item := range items {
		if item <= 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Ints(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneIntSlice(items []int) []int {
	if len(items) == 0 {
		return nil
	}
	out := make([]int, len(items))
	copy(out, items)
	return out
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
	if len(artifact.ServiceArtifacts) > 0 {
		return applyServiceArtifactsToBlueprintServices(blueprint, artifact.ServiceArtifacts)
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

func applyServiceArtifactsToBlueprintServices(blueprint *BlueprintRecord, artifacts []BuildServiceArtifactRecord) []string {
	if blueprint == nil || len(blueprint.Compiled.Services) == 0 || len(artifacts) == 0 {
		return nil
	}
	applied := make([]string, 0, len(artifacts))
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		matches := resolveServiceArtifactTargetIndexes(blueprint.Compiled.Services, artifact)
		if len(matches) != 1 {
			continue
		}
		index := matches[0]
		if index < 0 || index >= len(blueprint.Compiled.Services) {
			continue
		}
		service := blueprint.Compiled.Services[index]
		applyArtifactToBlueprintService(&service, BuildArtifactMetadataStageRecord{
			CommitSHA:            "",
			ArtifactRef:          artifact.ArtifactRef,
			ImageRef:             artifact.ImageRef,
			ImageDigest:          artifact.ImageDigest,
			DetectedPorts:        artifact.DetectedPorts,
			SuggestedTargetPort:  artifact.SuggestedTargetPort,
			DetectedFramework:    artifact.DetectedFramework,
			SuggestedHealthcheck: artifact.SuggestedHealthcheck,
		}, false)
		blueprint.Compiled.Services[index] = service
		if _, exists := seen[service.Name]; exists {
			continue
		}
		seen[service.Name] = struct{}{}
		applied = append(applied, service.Name)
	}
	return applied
}

func resolveServiceArtifactTargetIndex(services []BlueprintServiceContractRecord, artifact BuildServiceArtifactRecord) int {
	matches := resolveServiceArtifactTargetIndexes(services, artifact)
	if len(matches) == 1 {
		return matches[0]
	}
	return -1
}

func resolveServiceArtifactTargetIndexes(services []BlueprintServiceContractRecord, artifact BuildServiceArtifactRecord) []int {
	if len(services) == 0 {
		return nil
	}
	artifactName := normalizeArtifactServiceMatcher(artifact.ServiceName)
	artifactPath := normalizeArtifactServiceMatcher(artifact.ServicePath)
	if artifactName == "" && artifactPath == "" {
		return nil
	}

	matches := make([]int, 0, len(services))
	if artifactName != "" && artifactPath != "" {
		for index, service := range services {
			if normalizeArtifactServiceMatcher(service.Name) != artifactName {
				continue
			}
			if !matchesArtifactServicePath(service.Path, artifactPath) {
				continue
			}
			matches = append(matches, index)
		}
		return matches
	}
	if artifactName != "" {
		for index, service := range services {
			if normalizeArtifactServiceMatcher(service.Name) == artifactName {
				matches = append(matches, index)
			}
		}
		if len(matches) > 0 {
			return matches
		}
	}
	if artifactPath != "" {
		for index, service := range services {
			if matchesArtifactServicePath(service.Path, artifactPath) {
				matches = append(matches, index)
			}
		}
	}
	return matches
}

func matchesArtifactServicePath(servicePath, artifactPath string) bool {
	for _, candidate := range buildArtifactPathMatchers(servicePath) {
		if candidate == artifactPath {
			return true
		}
	}
	return false
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

	if healthPort := extractHealthcheckPort(service.Healthcheck); healthPort > 0 {
		if service.TargetPort <= 0 {
			service.TargetPort = healthPort
		}
		if service.ServicePort <= 0 {
			service.ServicePort = healthPort
		}
	}

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
