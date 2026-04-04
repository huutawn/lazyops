package service

import "time"

const (
	BuildJobStatusQueued    = "queued"
	BuildJobStatusRunning   = "running"
	BuildJobStatusSucceeded = "succeeded"
	BuildJobStatusFailed    = "failed"
	BuildJobStatusCanceled  = "canceled"

	DefaultBuildJobMaxAttempts = 3
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
	CommitSHA        string   `json:"commit_sha"`
	ArtifactRef      string   `json:"artifact_ref,omitempty"`
	ImageRef         string   `json:"image_ref,omitempty"`
	ImageDigest      string   `json:"image_digest,omitempty"`
	DetectedServices []string `json:"detected_services,omitempty"`
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

type BuildCallbackCommand struct {
	BuildJobID       string
	ProjectID        string
	CommitSHA        string
	Status           string
	ImageRef         string
	ImageDigest      string
	DetectedServices []string
}

type BuildCallbackResult struct {
	BuildJob BuildJobRecord
	Revision *DesiredStateRevisionRecord
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
