package request

import "encoding/json"

type CompileBlueprintArtifactMetadataRequest struct {
	CommitSHA   string `json:"commit_sha"`
	ArtifactRef string `json:"artifact_ref"`
	ImageRef    string `json:"image_ref"`
}

type CompileBlueprintRequest struct {
	SourceRef   string                                  `json:"source_ref"`
	TriggerKind string                                  `json:"trigger_kind"`
	Artifact    CompileBlueprintArtifactMetadataRequest `json:"artifact_metadata"`
	LazyopsYAML json.RawMessage                         `json:"lazyops_yaml"`
}
