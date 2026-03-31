package contracts

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Project struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id,omitempty"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

type GitHubInstallation struct {
	ID                   string         `json:"id"`
	UserID               string         `json:"user_id,omitempty"`
	GitHubInstallationID int64          `json:"github_installation_id,omitempty"`
	AccountLogin         string         `json:"account_login"`
	AccountType          string         `json:"account_type"`
	ScopeJSON            map[string]any `json:"scope_json,omitempty"`
	InstalledAt          time.Time      `json:"installed_at,omitempty"`
	RevokedAt            *time.Time     `json:"revoked_at,omitempty"`
}

type Instance struct {
	ID                      string         `json:"id"`
	UserID                  string         `json:"user_id,omitempty"`
	Name                    string         `json:"name"`
	PublicIP                string         `json:"public_ip,omitempty"`
	PrivateIP               string         `json:"private_ip,omitempty"`
	AgentID                 string         `json:"agent_id,omitempty"`
	Status                  string         `json:"status"`
	LabelsJSON              map[string]any `json:"labels_json,omitempty"`
	RuntimeCapabilitiesJSON map[string]any `json:"runtime_capabilities_json,omitempty"`
	CreatedAt               time.Time      `json:"created_at,omitempty"`
}

type MeshNetwork struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id,omitempty"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	CIDR      string    `json:"cidr,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type Cluster struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id,omitempty"`
	Name                string    `json:"name"`
	Provider            string    `json:"provider"`
	KubeconfigSecretRef string    `json:"kubeconfig_secret_ref,omitempty"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at,omitempty"`
}

type DeploymentBinding struct {
	ID                      string         `json:"id"`
	ProjectID               string         `json:"project_id"`
	Name                    string         `json:"name"`
	TargetRef               string         `json:"target_ref"`
	RuntimeMode             string         `json:"runtime_mode"`
	TargetKind              string         `json:"target_kind"`
	TargetID                string         `json:"target_id"`
	PlacementPolicyJSON     map[string]any `json:"placement_policy_json,omitempty"`
	DomainPolicyJSON        map[string]any `json:"domain_policy_json,omitempty"`
	CompatibilityPolicyJSON map[string]any `json:"compatibility_policy_json,omitempty"`
	ScaleToZeroPolicyJSON   map[string]any `json:"scale_to_zero_policy_json,omitempty"`
	CreatedAt               time.Time      `json:"created_at,omitempty"`
}

type ProjectsResponse struct {
	Projects []Project `json:"projects"`
}

type GitHubInstallationsResponse struct {
	Installations []GitHubInstallation `json:"installations"`
}

type InstancesResponse struct {
	Instances []Instance `json:"instances"`
}

type MeshNetworksResponse struct {
	MeshNetworks []MeshNetwork `json:"mesh_networks"`
}

type ClustersResponse struct {
	Clusters []Cluster `json:"clusters"`
}

type DeploymentBindingsResponse struct {
	Bindings []DeploymentBinding `json:"bindings"`
}

type TraceSummary struct {
	CorrelationID  string   `json:"correlation_id"`
	ServicePath    []string `json:"service_path"`
	NodeHops       []string `json:"node_hops,omitempty"`
	LatencyHotspot string   `json:"latency_hotspot,omitempty"`
	TotalLatencyMS int      `json:"total_latency_ms,omitempty"`
}

type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Node      string    `json:"node,omitempty"`
}

type LogsStreamPreview struct {
	Service string    `json:"service"`
	Cursor  string    `json:"cursor,omitempty"`
	Lines   []LogLine `json:"lines"`
}

func DecodeProjectsResponse(payload []byte) (ProjectsResponse, error) {
	var response ProjectsResponse
	if err := decode(payload, &response); err != nil {
		return ProjectsResponse{}, err
	}

	return response, response.Validate()
}

func DecodeGitHubInstallationsResponse(payload []byte) (GitHubInstallationsResponse, error) {
	var response GitHubInstallationsResponse
	if err := decode(payload, &response); err != nil {
		return GitHubInstallationsResponse{}, err
	}

	return response, response.Validate()
}

func DecodeInstancesResponse(payload []byte) (InstancesResponse, error) {
	var response InstancesResponse
	if err := decode(payload, &response); err != nil {
		return InstancesResponse{}, err
	}

	return response, response.Validate()
}

func DecodeMeshNetworksResponse(payload []byte) (MeshNetworksResponse, error) {
	var response MeshNetworksResponse
	if err := decode(payload, &response); err != nil {
		return MeshNetworksResponse{}, err
	}

	return response, response.Validate()
}

func DecodeClustersResponse(payload []byte) (ClustersResponse, error) {
	var response ClustersResponse
	if err := decode(payload, &response); err != nil {
		return ClustersResponse{}, err
	}

	return response, response.Validate()
}

func DecodeDeploymentBindingsResponse(payload []byte) (DeploymentBindingsResponse, error) {
	var response DeploymentBindingsResponse
	if err := decode(payload, &response); err != nil {
		return DeploymentBindingsResponse{}, err
	}

	return response, response.Validate()
}

func DecodeDeploymentBinding(payload []byte) (DeploymentBinding, error) {
	var binding DeploymentBinding
	if err := decode(payload, &binding); err != nil {
		return DeploymentBinding{}, err
	}

	return binding, binding.Validate()
}

func DecodeTraceSummary(payload []byte) (TraceSummary, error) {
	var trace TraceSummary
	if err := decode(payload, &trace); err != nil {
		return TraceSummary{}, err
	}

	return trace, trace.Validate()
}

func DecodeLogsStreamPreview(payload []byte) (LogsStreamPreview, error) {
	var preview LogsStreamPreview
	if err := decode(payload, &preview); err != nil {
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
