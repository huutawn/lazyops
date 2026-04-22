package response

import "time"

type ProjectEnvHelperSnippetResponse struct {
	Language  string `json:"language"`
	Framework string `json:"framework"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}

type ProjectEnvHelperPackResponse struct {
	ServiceKind      string                            `json:"service_kind"`
	Alias            string                            `json:"alias"`
	Category         string                            `json:"category"`
	Audience         string                            `json:"audience"`
	SourceService    string                            `json:"source_service"`
	RelatedServices  []string                          `json:"related_services"`
	PrimaryKey       string                            `json:"primary_key"`
	PublicPath       string                            `json:"public_path,omitempty"`
	Managed          bool                              `json:"managed"`
	RuntimeInjected  bool                              `json:"runtime_injected"`
	PlaceholderEnv   map[string]string                 `json:"placeholder_env"`
	EnvExample       map[string]string                 `json:"env_example"`
	LocalExampleEnv  map[string]string                 `json:"local_example_env"`
	RuntimeKeys      []string                          `json:"runtime_keys"`
	ProvisionedKeys  []string                          `json:"provisioned_keys"`
	Notes            []string                          `json:"notes"`
	LanguageSnippets []ProjectEnvHelperSnippetResponse `json:"language_snippets"`
}

type ProjectEnvBundleResponse struct {
	Configured      bool                           `json:"configured"`
	UpdatedAt       *time.Time                     `json:"updated_at,omitempty"`
	Fingerprint     string                         `json:"fingerprint,omitempty"`
	Keys            []string                       `json:"keys"`
	UserKeys        []string                       `json:"user_keys"`
	ManagedKeys     []string                       `json:"managed_keys"`
	ProvisionedKeys []string                       `json:"provisioned_keys"`
	ParseWarnings   []string                       `json:"parse_warnings"`
	HelperPacks     []ProjectEnvHelperPackResponse `json:"helper_packs"`
}
