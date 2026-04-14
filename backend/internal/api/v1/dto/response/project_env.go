package response

import "time"

type ProjectEnvHelperSnippetResponse struct {
	ServiceKind string            `json:"service_kind"`
	Alias       string            `json:"alias"`
	Env         map[string]string `json:"env"`
}

type ProjectEnvBundleResponse struct {
	Configured     bool                              `json:"configured"`
	UpdatedAt      *time.Time                        `json:"updated_at,omitempty"`
	Fingerprint    string                            `json:"fingerprint,omitempty"`
	Keys           []string                          `json:"keys"`
	ParseWarnings  []string                          `json:"parse_warnings"`
	HelperSnippets []ProjectEnvHelperSnippetResponse `json:"helper_snippets"`
}
