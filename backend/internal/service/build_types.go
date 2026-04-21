package service

import (
	"sort"
	"strings"
	"time"
)

const (
	BuildJobStatusQueued    = "queued"
	BuildJobStatusRunning   = "running"
	BuildJobStatusSucceeded = "succeeded"
	BuildJobStatusFailed    = "failed"
	BuildJobStatusCanceled  = "canceled"

	DefaultBuildJobMaxAttempts = 3

	BuildPortResolutionStatusResolved   = "resolved"
	BuildPortResolutionStatusAmbiguous  = "ambiguous"
	BuildPortResolutionStatusUnresolved = "unresolved"

	BuildPortResolutionSourceExplicit      = "explicit"
	BuildPortResolutionSourceDockerInspect = "docker_inspect"
	BuildPortResolutionSourceFrameworkHint = "framework_hint"
	BuildPortResolutionSourceSmokeRun      = "smoke_run"
	BuildPortResolutionSourceStartHint     = "start_hint"
	BuildPortResolutionSourceMixed         = "mixed"
	BuildPortResolutionSourceInternal      = "internal_default"
)

type BuildRetryPolicyRecord struct {
	MaxAttempts int    `json:"max_attempts"`
	Backoff     string `json:"backoff"`
}

type BuildCallbackExpectationRecord struct {
	Path           string   `json:"path"`
	RequiredFields []string `json:"required_fields"`
}

type BuildArtifactMetadataStageRecord struct {
	CommitSHA               string                           `json:"commit_sha"`
	ArtifactRef             string                           `json:"artifact_ref,omitempty"`
	ImageRef                string                           `json:"image_ref,omitempty"`
	ImageDigest             string                           `json:"image_digest,omitempty"`
	ServiceArtifacts        []BuildServiceArtifactRecord     `json:"service_artifacts,omitempty"`
	AppliedServices         []string                         `json:"applied_services,omitempty"`
	DetectedServices        []string                         `json:"detected_services,omitempty"`
	DetectedPorts           []ServiceDetectedPortRecord      `json:"detected_ports,omitempty"`
	PortDetectionSource     string                           `json:"port_detection_source,omitempty"`
	PortDetectionConfidence string                           `json:"port_detection_confidence,omitempty"`
	SuggestedTargetPort     int                              `json:"suggested_target_port,omitempty"`
	DetectedFramework       string                           `json:"detected_framework,omitempty"`
	SuggestedHealthcheck    *BuildSuggestedHealthcheckRecord `json:"suggested_healthcheck,omitempty"`
	PortResolutionStatus    string                           `json:"port_resolution_status,omitempty"`
	PortResolutionSource    string                           `json:"port_resolution_source,omitempty"`
	PortResolutionReason    string                           `json:"port_resolution_reason,omitempty"`
	CandidatePorts          []int                            `json:"candidate_ports,omitempty"`
}

type BuildSuggestedHealthcheckRecord struct {
	Path string `json:"path"`
	Port int    `json:"port"`
}

type BuildServiceArtifactRecord struct {
	ServiceName             string                           `json:"service_name"`
	ServicePath             string                           `json:"service_path"`
	ArtifactRef             string                           `json:"artifact_ref,omitempty"`
	ImageRef                string                           `json:"image_ref,omitempty"`
	ImageDigest             string                           `json:"image_digest,omitempty"`
	DetectedPorts           []ServiceDetectedPortRecord      `json:"detected_ports,omitempty"`
	PortDetectionSource     string                           `json:"port_detection_source,omitempty"`
	PortDetectionConfidence string                           `json:"port_detection_confidence,omitempty"`
	SuggestedTargetPort     int                              `json:"suggested_target_port,omitempty"`
	DetectedFramework       string                           `json:"detected_framework,omitempty"`
	SuggestedHealthcheck    *BuildSuggestedHealthcheckRecord `json:"suggested_healthcheck,omitempty"`
	PortResolutionStatus    string                           `json:"port_resolution_status,omitempty"`
	PortResolutionSource    string                           `json:"port_resolution_source,omitempty"`
	PortResolutionReason    string                           `json:"port_resolution_reason,omitempty"`
	CandidatePorts          []int                            `json:"candidate_ports,omitempty"`
}

type BuildTargetServiceRecord struct {
	ServiceName         string         `json:"service_name"`
	ServicePath         string         `json:"service_path"`
	RuntimeProfile      string         `json:"runtime_profile,omitempty"`
	Public              bool           `json:"public,omitempty"`
	StartHint           string         `json:"start_hint,omitempty"`
	DeclaredTargetPort  int            `json:"declared_target_port,omitempty"`
	DeclaredServicePort int            `json:"declared_service_port,omitempty"`
	DeclaredHealthcheck map[string]any `json:"declared_healthcheck,omitempty"`
}

type BuildWorkerInputRecord struct {
	BuildJobID            string                           `json:"build_job_id"`
	ProjectID             string                           `json:"project_id"`
	ProjectRepoLinkID     string                           `json:"project_repo_link_id"`
	GitHubDeliveryID      string                           `json:"github_delivery_id"`
	GitHubInstallationID  int64                            `json:"github_installation_id"`
	GitHubRepoID          int64                            `json:"github_repo_id"`
	RepoOwner             string                           `json:"repo_owner"`
	RepoName              string                           `json:"repo_name"`
	RepoFullName          string                           `json:"repo_full_name"`
	TrackedBranch         string                           `json:"tracked_branch"`
	CommitSHA             string                           `json:"commit_sha"`
	TriggerKind           string                           `json:"trigger_kind"`
	PullRequestNumber     int                              `json:"pull_request_number,omitempty"`
	PreviewEnabled        bool                             `json:"preview_enabled"`
	ServiceTargets        []BuildTargetServiceRecord       `json:"service_targets,omitempty"`
	ArtifactMetadataStage BuildArtifactMetadataStageRecord `json:"artifact_metadata_stage"`
	RetryPolicy           BuildRetryPolicyRecord           `json:"retry_policy"`
	CallbackExpectation   BuildCallbackExpectationRecord   `json:"callback_expectation"`
}

type BuildJobRecord struct {
	ID                   string
	ProjectID            string
	ProjectRepoLinkID    string
	GitHubDeliveryID     string
	GitHubInstallationID int64
	GitHubRepoID         int64
	RepoFullName         string
	TriggerKind          string
	Status               string
	CommitSHA            string
	TrackedBranch        string
	PullRequestNumber    int
	RetryCount           int
	MaxAttempts          int
	WorkerInput          BuildWorkerInputRecord
	ArtifactMetadata     BuildArtifactMetadataStageRecord
	StartedAt            *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ManualBuildEnqueueCommand struct {
	ProjectID            string
	ProjectRepoLinkID    string
	GitHubInstallationID int64
	GitHubRepoID         int64
	RepoOwner            string
	RepoName             string
	RepoFullName         string
	TrackedBranch        string
	CommitSHA            string
	TriggerKind          string
	PreviewEnabled       bool
}

type BuildCallbackCommand struct {
	BuildJobID              string
	ProjectID               string
	CommitSHA               string
	Status                  string
	ImageRef                string
	ImageDigest             string
	ServiceArtifacts        []BuildServiceArtifactRecord
	DetectedServices        []string
	DetectedPorts           []ServiceDetectedPortRecord
	PortDetectionSource     string
	PortDetectionConfidence string
	SuggestedTargetPort     int
	DetectedFramework       string
	SuggestedHealthcheck    *BuildSuggestedHealthcheckRecord
	PortResolutionStatus    string
	PortResolutionSource    string
	PortResolutionReason    string
	CandidatePorts          []int
}

type BuildCallbackResult struct {
	BuildJob   BuildJobRecord
	Revision   *DesiredStateRevisionRecord
	Deployment *DeploymentRecord
}

type BuildRealtimeEvent struct {
	Type    string       `json:"type"`
	Payload any          `json:"payload"`
	Meta    RealtimeMeta `json:"meta"`
}

type BuildFailureRealtimePayload struct {
	BuildJobID       string                           `json:"build_job_id"`
	ProjectID        string                           `json:"project_id"`
	Status           string                           `json:"status"`
	TriggerKind      string                           `json:"trigger_kind"`
	CommitSHA        string                           `json:"commit_sha"`
	TrackedBranch    string                           `json:"tracked_branch"`
	ArtifactMetadata BuildArtifactMetadataStageRecord `json:"artifact_metadata"`
}

func normalizeBuildTargetServices(items []BuildTargetServiceRecord) []BuildTargetServiceRecord {
	if len(items) == 0 {
		return nil
	}
	out := make([]BuildTargetServiceRecord, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ServiceName)
		path := strings.TrimSpace(item.ServicePath)
		if name == "" || path == "" {
			continue
		}
		key := buildTargetServiceKey(name, path)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, BuildTargetServiceRecord{
			ServiceName:         name,
			ServicePath:         path,
			RuntimeProfile:      strings.TrimSpace(item.RuntimeProfile),
			Public:              item.Public,
			StartHint:           strings.TrimSpace(item.StartHint),
			DeclaredTargetPort:  item.DeclaredTargetPort,
			DeclaredServicePort: item.DeclaredServicePort,
			DeclaredHealthcheck: cloneAnyMap(item.DeclaredHealthcheck),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ServicePath != out[j].ServicePath {
			return out[i].ServicePath < out[j].ServicePath
		}
		return out[i].ServiceName < out[j].ServiceName
	})
	return out
}

func buildTargetServiceKey(serviceName, servicePath string) string {
	return normalizeArtifactServiceMatcher(serviceName) + "|" + normalizeArtifactServiceMatcher(servicePath)
}

func buildTargetServiceSummaries(items []BuildTargetServiceRecord) []string {
	normalized := normalizeBuildTargetServices(items)
	if len(normalized) == 0 {
		return nil
	}
	out := make([]string, 0, len(normalized))
	for _, item := range normalized {
		out = append(out, strings.TrimSpace(item.ServiceName)+"@"+strings.TrimSpace(item.ServicePath))
	}
	return out
}

func buildCallbackRequiredFields(serviceTargets []BuildTargetServiceRecord) []string {
	fields := []string{
		"build_job_id",
		"project_id",
		"commit_sha",
		"status",
		"metadata.detected_services",
	}
	if len(normalizeBuildTargetServices(serviceTargets)) > 1 {
		return append(fields, "metadata.service_artifacts")
	}
	return append(fields, "image_ref", "image_digest")
}
