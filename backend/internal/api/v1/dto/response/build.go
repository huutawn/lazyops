package response

import "time"

type BuildArtifactMetadataResponse struct {
	CommitSHA               string                             `json:"commit_sha"`
	ArtifactRef             string                             `json:"artifact_ref,omitempty"`
	ImageRef                string                             `json:"image_ref,omitempty"`
	ImageDigest             string                             `json:"image_digest,omitempty"`
	ServiceArtifacts        []BuildServiceArtifactResponse     `json:"service_artifacts,omitempty"`
	AppliedServices         []string                           `json:"applied_services,omitempty"`
	DetectedServices        []string                           `json:"detected_services,omitempty"`
	DetectedPorts           []BuildDetectedPortResponse        `json:"detected_ports,omitempty"`
	PortDetectionSource     string                             `json:"port_detection_source,omitempty"`
	PortDetectionConfidence string                             `json:"port_detection_confidence,omitempty"`
	SuggestedTargetPort     int                                `json:"suggested_target_port,omitempty"`
	DetectedFramework       string                             `json:"detected_framework,omitempty"`
	SuggestedHealthcheck    *BuildSuggestedHealthcheckResponse `json:"suggested_healthcheck,omitempty"`
	PortResolutionStatus    string                             `json:"port_resolution_status,omitempty"`
	PortResolutionSource    string                             `json:"port_resolution_source,omitempty"`
	PortResolutionReason    string                             `json:"port_resolution_reason,omitempty"`
	CandidatePorts          []int                              `json:"candidate_ports,omitempty"`
}

type BuildDetectedPortResponse struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
	Exposed  bool   `json:"exposed,omitempty"`
}

type BuildSuggestedHealthcheckResponse struct {
	Path string `json:"path"`
	Port int    `json:"port"`
}

type BuildServiceArtifactResponse struct {
	ServiceName             string                             `json:"service_name"`
	ServicePath             string                             `json:"service_path"`
	ArtifactRef             string                             `json:"artifact_ref,omitempty"`
	ImageRef                string                             `json:"image_ref,omitempty"`
	ImageDigest             string                             `json:"image_digest,omitempty"`
	DetectedPorts           []BuildDetectedPortResponse        `json:"detected_ports,omitempty"`
	PortDetectionSource     string                             `json:"port_detection_source,omitempty"`
	PortDetectionConfidence string                             `json:"port_detection_confidence,omitempty"`
	SuggestedTargetPort     int                                `json:"suggested_target_port,omitempty"`
	DetectedFramework       string                             `json:"detected_framework,omitempty"`
	SuggestedHealthcheck    *BuildSuggestedHealthcheckResponse `json:"suggested_healthcheck,omitempty"`
	PortResolutionStatus    string                             `json:"port_resolution_status,omitempty"`
	PortResolutionSource    string                             `json:"port_resolution_source,omitempty"`
	PortResolutionReason    string                             `json:"port_resolution_reason,omitempty"`
	CandidatePorts          []int                              `json:"candidate_ports,omitempty"`
}

type BuildJobResponse struct {
	ID                string                        `json:"id"`
	ProjectID         string                        `json:"project_id"`
	ProjectRepoLinkID string                        `json:"project_repo_link_id"`
	GitHubDeliveryID  string                        `json:"github_delivery_id"`
	TriggerKind       string                        `json:"trigger_kind"`
	Status            string                        `json:"status"`
	CommitSHA         string                        `json:"commit_sha"`
	TrackedBranch     string                        `json:"tracked_branch"`
	PullRequestNumber int                           `json:"pull_request_number,omitempty"`
	RetryCount        int                           `json:"retry_count"`
	MaxAttempts       int                           `json:"max_attempts"`
	ArtifactMetadata  BuildArtifactMetadataResponse `json:"artifact_metadata"`
	StartedAt         *time.Time                    `json:"started_at,omitempty"`
	CompletedAt       *time.Time                    `json:"completed_at,omitempty"`
	CreatedAt         time.Time                     `json:"created_at"`
	UpdatedAt         time.Time                     `json:"updated_at"`
}

type BuildCallbackResponse struct {
	BuildJob BuildJobResponse              `json:"build_job"`
	Revision *DesiredStateRevisionResponse `json:"revision,omitempty"`
}
