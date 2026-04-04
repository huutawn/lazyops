package request

type BuildCallbackMetadataRequest struct {
	DetectedServices []string `json:"detected_services"`
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
