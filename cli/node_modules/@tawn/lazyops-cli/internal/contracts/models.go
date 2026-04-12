package contracts

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type Project struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GitHubInstallation struct {
	ID                   string         `json:"id"`
	GitHubInstallationID int64          `json:"github_installation_id"`
	AccountLogin         string         `json:"account_login"`
	AccountType          string         `json:"account_type"`
	ScopeJSON            map[string]any `json:"scope"`
	InstalledAt          time.Time      `json:"installed_at"`
	RevokedAt            *time.Time     `json:"revoked_at"`
}

type Instance struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	PublicIP            string         `json:"public_ip"`
	PrivateIP           string         `json:"private_ip"`
	AgentID             string         `json:"agent_id"`
	Status              string         `json:"status"`
	Labels              map[string]any `json:"labels"`
	RuntimeCapabilities map[string]any `json:"runtime_capabilities"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type MeshNetwork struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	CIDR      string    `json:"cidr"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Cluster struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Provider            string    `json:"provider"`
	KubeconfigSecretRef string    `json:"kubeconfig_secret_ref"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type DeploymentBinding struct {
	ID                  string         `json:"id"`
	ProjectID           string         `json:"project_id"`
	Name                string         `json:"name"`
	TargetRef           string         `json:"target_ref"`
	RuntimeMode         string         `json:"runtime_mode"`
	TargetKind          string         `json:"target_kind"`
	TargetID            string         `json:"target_id"`
	PlacementPolicy     map[string]any `json:"placement_policy"`
	DomainPolicy        map[string]any `json:"domain_policy"`
	CompatibilityPolicy map[string]any `json:"compatibility_policy"`
	ScaleToZeroPolicy   map[string]any `json:"scale_to_zero_policy"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type ProjectsResponse struct {
	Projects []Project `json:"items"`
}

type GitHubInstallationsResponse struct {
	Installations []GitHubInstallation `json:"items"`
}

type InstancesResponse struct {
	Instances []Instance `json:"items"`
}

type MeshNetworksResponse struct {
	MeshNetworks []MeshNetwork `json:"items"`
}

type ClustersResponse struct {
	Clusters []Cluster `json:"items"`
}

type DeploymentBindingsResponse struct {
	Bindings []DeploymentBinding `json:"items"`
}

type InitTargetSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	RuntimeMode string `json:"runtime_mode"`
}

type LazyopsYAMLSchema struct {
	AllowedDependencyProtocols  []string `json:"allowed_dependency_protocols"`
	AllowedMagicDomainProviders []string `json:"allowed_magic_domain_providers"`
	ForbiddenFieldNames         []string `json:"forbidden_field_names"`
}

type ValidateLazyopsYAMLResponse struct {
	Project           Project           `json:"project"`
	DeploymentBinding DeploymentBinding `json:"deployment_binding"`
	TargetSummary     InitTargetSummary `json:"target_summary"`
	Schema            LazyopsYAMLSchema `json:"schema"`
}

type ProjectRepoLink struct {
	ID                   string    `json:"id"`
	ProjectID            string    `json:"project_id"`
	GitHubInstallationID int64     `json:"github_installation_id"`
	GitHubRepoID         int64     `json:"github_repo_id"`
	RepoOwner            string    `json:"repo_owner"`
	RepoName             string    `json:"repo_name"`
	RepoFullName         string    `json:"repo_full_name"`
	TrackedBranch        string    `json:"tracked_branch"`
	PreviewEnabled       bool      `json:"preview_enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type TraceSummary struct {
	CorrelationID  string   `json:"correlation_id"`
	ServicePath    []string `json:"service_path"`
	NodeHops       []string `json:"node_hops"`
	LatencyHotspot string   `json:"latency_hotspot"`
	TotalLatencyMS int      `json:"total_latency_ms"`
}

type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Node      string    `json:"node"`
}

type LogsStreamPreview struct {
	Service string    `json:"service"`
	Cursor  string    `json:"cursor"`
	Lines   []LogLine `json:"lines"`
}

type TunnelSession struct {
	SessionID string `json:"session_id"`
	LocalPort int    `json:"local_port"`
	Remote    string `json:"remote"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at"`
}

type CLILoginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	TokenID   string `json:"token_id"`
	ExpiresAt string `json:"expires_at"`
	User      struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Status      string `json:"status"`
	} `json:"user"`
}

type PATRevokeResponse struct {
	TokenID string `json:"token_id"`
	Revoked bool   `json:"revoked"`
}

func DecodeEnvelope(payload []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	return env, nil
}

func DecodeFromEnvelope[T any](payload []byte, target *T) error {
	env, err := DecodeEnvelope(payload)
	if err != nil {
		return err
	}
	if !env.Success {
		type errorDetail struct {
			Code    string            `json:"code"`
			Details string            `json:"details"`
			Fields  map[string]string `json:"fields"`
		}
		var envErr struct {
			Error errorDetail `json:"error"`
		}
		if err := json.Unmarshal(payload, &envErr); err == nil && envErr.Error.Code != "" {
			return fmt.Errorf("api error (%s): %s", envErr.Error.Code, envErr.Error.Details)
		}
		return fmt.Errorf("api error: %s", env.Message)
	}
	if len(env.Data) == 0 {
		return nil
	}
	return json.Unmarshal(env.Data, target)
}

func DecodeTunnelSession(payload []byte) (TunnelSession, error) {
	var session TunnelSession
	if err := DecodeFromEnvelope(payload, &session); err != nil {
		return TunnelSession{}, err
	}
	return session, session.Validate()
}

func (session TunnelSession) Validate() error {
	if err := requireValue("tunnel_session.session_id", session.SessionID); err != nil {
		return err
	}
	if session.LocalPort <= 0 {
		return fmt.Errorf("tunnel_session.local_port must be greater than zero")
	}
	if err := requireValue("tunnel_session.status", session.Status); err != nil {
		return err
	}
	return nil
}

func DecodeCLILoginResponse(payload []byte) (CLILoginResponse, error) {
	var resp CLILoginResponse
	if err := DecodeFromEnvelope(payload, &resp); err != nil {
		return CLILoginResponse{}, err
	}
	return resp, nil
}

func DecodePATRevokeResponse(payload []byte) (PATRevokeResponse, error) {
	var resp PATRevokeResponse
	if err := DecodeFromEnvelope(payload, &resp); err != nil {
		return PATRevokeResponse{}, err
	}
	return resp, nil
}

func DecodeProjectsResponse(payload []byte) (ProjectsResponse, error) {
	var response ProjectsResponse
	if err := DecodeFromEnvelope(payload, &response); err != nil {
		return ProjectsResponse{}, err
	}
	return response, response.Validate()
}

func DecodeGitHubInstallationsResponse(payload []byte) (GitHubInstallationsResponse, error) {
	var response GitHubInstallationsResponse
	if err := DecodeFromEnvelope(payload, &response); err != nil {
		return GitHubInstallationsResponse{}, err
	}
	return response, response.Validate()
}

func DecodeInstancesResponse(payload []byte) (InstancesResponse, error) {
	var response InstancesResponse
	if err := DecodeFromEnvelope(payload, &response); err != nil {
		return InstancesResponse{}, err
	}
	return response, response.Validate()
}

func DecodeMeshNetworksResponse(payload []byte) (MeshNetworksResponse, error) {
	var response MeshNetworksResponse
	if err := DecodeFromEnvelope(payload, &response); err != nil {
		return MeshNetworksResponse{}, err
	}
	return response, response.Validate()
}

func DecodeClustersResponse(payload []byte) (ClustersResponse, error) {
	var response ClustersResponse
	if err := DecodeFromEnvelope(payload, &response); err != nil {
		return ClustersResponse{}, err
	}
	return response, response.Validate()
}

func DecodeDeploymentBindingsResponse(payload []byte) (DeploymentBindingsResponse, error) {
	var response DeploymentBindingsResponse
	if err := DecodeFromEnvelope(payload, &response); err != nil {
		return DeploymentBindingsResponse{}, err
	}
	return response, response.Validate()
}

func DecodeDeploymentBinding(payload []byte) (DeploymentBinding, error) {
	var binding DeploymentBinding
	if err := DecodeFromEnvelope(payload, &binding); err != nil {
		return DeploymentBinding{}, err
	}
	return binding, binding.Validate()
}

func DecodeValidateLazyopsYAMLResponse(payload []byte) (ValidateLazyopsYAMLResponse, error) {
	var response ValidateLazyopsYAMLResponse
	if err := DecodeFromEnvelope(payload, &response); err != nil {
		return ValidateLazyopsYAMLResponse{}, err
	}
	return response, response.Validate()
}

func DecodeProjectRepoLink(payload []byte) (ProjectRepoLink, error) {
	var link ProjectRepoLink
	if err := DecodeFromEnvelope(payload, &link); err != nil {
		return ProjectRepoLink{}, err
	}
	return link, link.Validate()
}

func DecodeTraceSummary(payload []byte) (TraceSummary, error) {
	var trace TraceSummary
	if err := DecodeFromEnvelope(payload, &trace); err != nil {
		return TraceSummary{}, err
	}
	return trace, trace.Validate()
}

func DecodeLogsStreamPreview(payload []byte) (LogsStreamPreview, error) {
	var preview LogsStreamPreview
	if err := DecodeFromEnvelope(payload, &preview); err != nil {
		return LogsStreamPreview{}, err
	}
	return preview, preview.Validate()
}

func (project Project) Validate() error {
	if err := requireValue("project.id", project.ID); err != nil {
		return err
	}
	if err := requireValue("project.name", project.Name); err != nil {
		return err
	}
	return requireValue("project.slug", project.Slug)
}

func (installation GitHubInstallation) Validate() error {
	if err := requireValue("github_installation.id", installation.ID); err != nil {
		return err
	}
	if err := requireValue("github_installation.account_login", installation.AccountLogin); err != nil {
		return err
	}
	return requireValue("github_installation.account_type", installation.AccountType)
}

func (instance Instance) Validate() error {
	if err := requireValue("instance.id", instance.ID); err != nil {
		return err
	}
	if err := requireValue("instance.name", instance.Name); err != nil {
		return err
	}
	return requireValue("instance.status", instance.Status)
}

func (mesh MeshNetwork) Validate() error {
	if err := requireValue("mesh_network.id", mesh.ID); err != nil {
		return err
	}
	if err := requireValue("mesh_network.name", mesh.Name); err != nil {
		return err
	}
	if err := requireValue("mesh_network.provider", mesh.Provider); err != nil {
		return err
	}
	if !isOneOf(mesh.Provider, "wireguard", "tailscale") {
		return fmt.Errorf("mesh_network.provider must be wireguard or tailscale")
	}
	return requireValue("mesh_network.status", mesh.Status)
}

func (cluster Cluster) Validate() error {
	if err := requireValue("cluster.id", cluster.ID); err != nil {
		return err
	}
	if err := requireValue("cluster.name", cluster.Name); err != nil {
		return err
	}
	if err := requireValue("cluster.provider", cluster.Provider); err != nil {
		return err
	}
	if !isOneOf(cluster.Provider, "k3s") {
		return fmt.Errorf("cluster.provider must be k3s")
	}
	return requireValue("cluster.status", cluster.Status)
}

func (binding DeploymentBinding) Validate() error {
	if err := requireValue("deployment_binding.id", binding.ID); err != nil {
		return err
	}
	if err := requireValue("deployment_binding.project_id", binding.ProjectID); err != nil {
		return err
	}
	if err := requireValue("deployment_binding.name", binding.Name); err != nil {
		return err
	}
	if err := requireValue("deployment_binding.target_ref", binding.TargetRef); err != nil {
		return err
	}
	if err := requireValue("deployment_binding.runtime_mode", binding.RuntimeMode); err != nil {
		return err
	}
	if !isOneOf(binding.RuntimeMode, "standalone", "distributed-mesh", "distributed-k3s") {
		return fmt.Errorf("deployment_binding.runtime_mode is invalid")
	}
	if err := requireValue("deployment_binding.target_kind", binding.TargetKind); err != nil {
		return err
	}
	if !isOneOf(binding.TargetKind, "instance", "mesh", "cluster") {
		return fmt.Errorf("deployment_binding.target_kind is invalid")
	}
	return requireValue("deployment_binding.target_id", binding.TargetID)
}

func (response ProjectsResponse) Validate() error {
	for _, project := range response.Projects {
		if err := project.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (response GitHubInstallationsResponse) Validate() error {
	for _, installation := range response.Installations {
		if err := installation.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (response InstancesResponse) Validate() error {
	for _, instance := range response.Instances {
		if err := instance.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (response MeshNetworksResponse) Validate() error {
	for _, network := range response.MeshNetworks {
		if err := network.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (response ClustersResponse) Validate() error {
	for _, cluster := range response.Clusters {
		if err := cluster.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (response DeploymentBindingsResponse) Validate() error {
	for _, binding := range response.Bindings {
		if err := binding.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (summary InitTargetSummary) Validate() error {
	if err := requireValue("target_summary.id", summary.ID); err != nil {
		return err
	}
	if err := requireValue("target_summary.name", summary.Name); err != nil {
		return err
	}
	if err := requireValue("target_summary.kind", summary.Kind); err != nil {
		return err
	}
	if !isOneOf(summary.Kind, "instance", "mesh", "cluster") {
		return fmt.Errorf("target_summary.kind is invalid")
	}
	if err := requireValue("target_summary.status", summary.Status); err != nil {
		return err
	}
	if err := requireValue("target_summary.runtime_mode", summary.RuntimeMode); err != nil {
		return err
	}
	if !isOneOf(summary.RuntimeMode, "standalone", "distributed-mesh", "distributed-k3s") {
		return fmt.Errorf("target_summary.runtime_mode is invalid")
	}
	return nil
}

func (schema LazyopsYAMLSchema) Validate() error {
	if len(schema.AllowedDependencyProtocols) == 0 {
		return fmt.Errorf("lazyops_yaml_schema.allowed_dependency_protocols must not be empty")
	}
	if len(schema.AllowedMagicDomainProviders) == 0 {
		return fmt.Errorf("lazyops_yaml_schema.allowed_magic_domain_providers must not be empty")
	}
	if len(schema.ForbiddenFieldNames) == 0 {
		return fmt.Errorf("lazyops_yaml_schema.forbidden_field_names must not be empty")
	}
	return nil
}

func (response ValidateLazyopsYAMLResponse) Validate() error {
	if err := response.Project.Validate(); err != nil {
		return err
	}
	if err := response.DeploymentBinding.Validate(); err != nil {
		return err
	}
	if err := response.TargetSummary.Validate(); err != nil {
		return err
	}
	return response.Schema.Validate()
}

func (link ProjectRepoLink) Validate() error {
	if err := requireValue("project_repo_link.id", link.ID); err != nil {
		return err
	}
	if err := requireValue("project_repo_link.project_id", link.ProjectID); err != nil {
		return err
	}
	if link.GitHubInstallationID <= 0 {
		return fmt.Errorf("project_repo_link.github_installation_id must be greater than zero")
	}
	if link.GitHubRepoID <= 0 {
		return fmt.Errorf("project_repo_link.github_repo_id must be greater than zero")
	}
	if err := requireValue("project_repo_link.repo_owner", link.RepoOwner); err != nil {
		return err
	}
	if err := requireValue("project_repo_link.repo_name", link.RepoName); err != nil {
		return err
	}
	return requireValue("project_repo_link.tracked_branch", link.TrackedBranch)
}

func (trace TraceSummary) Validate() error {
	if err := requireValue("trace.correlation_id", trace.CorrelationID); err != nil {
		return err
	}
	if len(trace.ServicePath) == 0 {
		return fmt.Errorf("trace.service_path must not be empty")
	}
	return nil
}

func (preview LogsStreamPreview) Validate() error {
	if err := requireValue("logs.service", preview.Service); err != nil {
		return err
	}
	for index, line := range preview.Lines {
		if strings.TrimSpace(line.Message) == "" {
			return fmt.Errorf("logs.lines[%d].message must not be empty", index)
		}
		if strings.TrimSpace(line.Level) == "" {
			return fmt.Errorf("logs.lines[%d].level must not be empty", index)
		}
	}
	return nil
}

func decode(payload []byte, target any) error {
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode contract payload: %w", err)
	}
	return nil
}

func requireValue(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func isOneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(value), candidate) {
			return true
		}
	}
	return false
}
