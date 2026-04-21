package request

type BuildCallbackMetadataRequest struct {
	ServiceArtifacts        []BuildServiceArtifactRequest     `json:"service_artifacts,omitempty"`
	DetectedServices        []string                          `json:"detected_services"`
	DetectedPorts           []BuildDetectedPortRequest        `json:"detected_ports,omitempty"`
	PortDetectionSource     string                            `json:"port_detection_source,omitempty"`
	PortDetectionConfidence string                            `json:"port_detection_confidence,omitempty"`
	SuggestedTargetPort     int                               `json:"suggested_target_port,omitempty"`
	DetectedFramework       string                            `json:"detected_framework,omitempty"`
	SuggestedHealthcheck    *BuildSuggestedHealthcheckRequest `json:"suggested_healthcheck,omitempty"`
	PortResolutionStatus    string                            `json:"port_resolution_status,omitempty"`
	PortResolutionSource    string                            `json:"port_resolution_source,omitempty"`
	PortResolutionReason    string                            `json:"port_resolution_reason,omitempty"`
	CandidatePorts          []int                             `json:"candidate_ports,omitempty"`
}

type BuildServiceArtifactRequest struct {
	ServiceName             string                            `json:"service_name"`
	ServicePath             string                            `json:"service_path"`
	ImageRef                string                            `json:"image_ref"`
	ImageDigest             string                            `json:"image_digest"`
	DetectedPorts           []BuildDetectedPortRequest        `json:"detected_ports,omitempty"`
	PortDetectionSource     string                            `json:"port_detection_source,omitempty"`
	PortDetectionConfidence string                            `json:"port_detection_confidence,omitempty"`
	SuggestedTargetPort     int                               `json:"suggested_target_port,omitempty"`
	DetectedFramework       string                            `json:"detected_framework,omitempty"`
	SuggestedHealthcheck    *BuildSuggestedHealthcheckRequest `json:"suggested_healthcheck,omitempty"`
	PortResolutionStatus    string                            `json:"port_resolution_status,omitempty"`
	PortResolutionSource    string                            `json:"port_resolution_source,omitempty"`
	PortResolutionReason    string                            `json:"port_resolution_reason,omitempty"`
	CandidatePorts          []int                             `json:"candidate_ports,omitempty"`
}

type BuildDetectedPortRequest struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
	Exposed  bool   `json:"exposed,omitempty"`
}

type BuildSuggestedHealthcheckRequest struct {
	Path string `json:"path"`
	Port int    `json:"port"`
}

type BuildCallbackRequest struct {
	BuildJobID  string                       `json:"build_job_id"`
	ProjectID   string                       `json:"project_id"`
	CommitSHA   string                       `json:"commit_sha"`
	Status      string                       `json:"status"`
	ImageRef    string                       `json:"image_ref"`
	ImageDigest string                       `json:"image_digest"`
	Metadata    BuildCallbackMetadataRequest `json:"metadata"`
}
