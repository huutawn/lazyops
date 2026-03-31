package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/state"
)

var ErrNotImplemented = fmt.Errorf("runtime driver operation is not implemented yet")

type Driver interface {
	PrepareReleaseWorkspace(context.Context, RuntimeContext) (PreparedWorkspace, error)
	StartReleaseCandidate(context.Context, RuntimeContext) error
	RunHealthGate(context.Context, RuntimeContext) error
	PromoteRelease(context.Context, RuntimeContext) error
	RollbackRelease(context.Context, RuntimeContext) error
	SleepService(context.Context, RuntimeContext, string) error
	WakeService(context.Context, RuntimeContext, string) error
	GarbageCollectRuntime(context.Context, RuntimeContext) error
}

type RuntimeContext struct {
	Project  ProjectMetadata                    `json:"project"`
	Binding  contracts.DeploymentBindingPayload `json:"binding"`
	Revision contracts.DesiredRevisionPayload   `json:"revision"`
	Services []ServiceRuntimeContext            `json:"services"`
}

type ProjectMetadata struct {
	ProjectID string            `json:"project_id"`
	Name      string            `json:"name,omitempty"`
	Slug      string            `json:"slug,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type ServiceRuntimeContext struct {
	Name           string                               `json:"name"`
	Path           string                               `json:"path"`
	Public         bool                                 `json:"public"`
	RuntimeProfile string                               `json:"runtime_profile,omitempty"`
	StartHint      string                               `json:"start_hint,omitempty"`
	Labels         map[string]string                    `json:"labels,omitempty"`
	HealthCheck    contracts.HealthCheckPayload         `json:"healthcheck"`
	Dependencies   []contracts.DependencyBindingPayload `json:"dependencies,omitempty"`
	Placement      *contracts.PlacementAssignment       `json:"placement,omitempty"`
}

type WorkspaceLayout struct {
	Root      string `json:"root"`
	Artifacts string `json:"artifacts"`
	Config    string `json:"config"`
	Sidecars  string `json:"sidecars"`
	Gateway   string `json:"gateway"`
	Services  string `json:"services"`
}

type PreparedWorkspace struct {
	Layout           WorkspaceLayout   `json:"layout"`
	ManifestPath     string            `json:"manifest_path"`
	ServiceManifests map[string]string `json:"service_manifests"`
}

type WorkspaceManifest struct {
	PreparedAt   time.Time                          `json:"prepared_at"`
	Project      ProjectMetadata                    `json:"project"`
	Binding      contracts.DeploymentBindingPayload `json:"binding"`
	Revision     contracts.DesiredRevisionPayload   `json:"revision"`
	Services     []ServiceRuntimeContext            `json:"services"`
	Layout       WorkspaceLayout                    `json:"layout"`
	ArtifactPlan ArtifactPlan                       `json:"artifact_plan"`
	GatewayPlan  GatewayPlan                        `json:"gateway_plan"`
	SidecarPlan  SidecarPlan                        `json:"sidecar_plan"`
}

type ArtifactPlan struct {
	Status     string `json:"status"`
	RevisionID string `json:"revision_id"`
}

type GatewayPlan struct {
	Provider       string   `json:"provider"`
	PublicServices []string `json:"public_services"`
	MagicDomain    string   `json:"magic_domain_provider,omitempty"`
}

type SidecarPlan struct {
	EnabledServices []string                      `json:"enabled_services"`
	Compatibility   contracts.CompatibilityPolicy `json:"compatibility_policy"`
}

func ContextFromPreparePayload(payload contracts.PrepareReleaseWorkspacePayload) (RuntimeContext, error) {
	if strings.TrimSpace(payload.Project.ProjectID) == "" {
		return RuntimeContext{}, fmt.Errorf("project.project_id is required")
	}
	if strings.TrimSpace(payload.Binding.BindingID) == "" {
		return RuntimeContext{}, fmt.Errorf("binding.binding_id is required")
	}
	if strings.TrimSpace(payload.Revision.RevisionID) == "" {
		return RuntimeContext{}, fmt.Errorf("revision.revision_id is required")
	}
	if payload.Binding.ProjectID != "" && payload.Binding.ProjectID != payload.Project.ProjectID {
		return RuntimeContext{}, fmt.Errorf("binding project ID does not match project metadata")
	}
	if payload.Revision.ProjectID != "" && payload.Revision.ProjectID != payload.Project.ProjectID {
		return RuntimeContext{}, fmt.Errorf("revision project ID does not match project metadata")
	}
	if payload.Binding.RuntimeMode != payload.Revision.RuntimeMode {
		return RuntimeContext{}, fmt.Errorf("binding runtime mode does not match revision runtime mode")
	}
	switch payload.Binding.RuntimeMode {
	case contracts.RuntimeModeStandalone, contracts.RuntimeModeDistributedMesh:
	default:
		return RuntimeContext{}, fmt.Errorf("runtime driver does not support runtime mode %q", payload.Binding.RuntimeMode)
	}
	if len(payload.Revision.Services) == 0 {
		return RuntimeContext{}, fmt.Errorf("revision must include at least one service")
	}

	serviceNames := make(map[string]struct{}, len(payload.Revision.Services))
	services := make([]ServiceRuntimeContext, 0, len(payload.Revision.Services))
	for _, service := range payload.Revision.Services {
		if strings.TrimSpace(service.Name) == "" {
			return RuntimeContext{}, fmt.Errorf("service name is required")
		}
		if _, exists := serviceNames[service.Name]; exists {
			return RuntimeContext{}, fmt.Errorf("duplicate service name %q", service.Name)
		}
		serviceNames[service.Name] = struct{}{}

		ctxService := ServiceRuntimeContext{
			Name:           service.Name,
			Path:           service.Path,
			Public:         service.Public,
			RuntimeProfile: service.RuntimeProfile,
			StartHint:      service.StartHint,
			Labels:         service.Labels,
			HealthCheck:    service.HealthCheck,
		}

		for _, dependency := range payload.Revision.DependencyBindings {
			if dependency.Service == service.Name {
				ctxService.Dependencies = append(ctxService.Dependencies, dependency)
			}
		}
		for _, placement := range payload.Revision.PlacementAssignments {
			if placement.ServiceName == service.Name {
				placementCopy := placement
				ctxService.Placement = &placementCopy
				break
			}
		}
		services = append(services, ctxService)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return RuntimeContext{
		Project: ProjectMetadata{
			ProjectID: payload.Project.ProjectID,
			Name:      payload.Project.Name,
			Slug:      payload.Project.Slug,
			Labels:    payload.Project.Labels,
		},
		Binding:  payload.Binding,
		Revision: payload.Revision,
		Services: services,
	}, nil
}

func workspaceLayout(root string, runtimeCtx RuntimeContext) WorkspaceLayout {
	workspaceRoot := filepath.Join(
		root,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
		"revisions",
		runtimeCtx.Revision.RevisionID,
	)
	return WorkspaceLayout{
		Root:      workspaceRoot,
		Artifacts: filepath.Join(workspaceRoot, "artifacts"),
		Config:    filepath.Join(workspaceRoot, "config"),
		Sidecars:  filepath.Join(workspaceRoot, "sidecars"),
		Gateway:   filepath.Join(workspaceRoot, "gateway"),
		Services:  filepath.Join(workspaceRoot, "services"),
	}
}

func capabilityNoContainerLeak(manifest WorkspaceManifest) error {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(string(raw)), "container") {
		return fmt.Errorf("runtime workspace manifest must not leak container terminology")
	}
	return nil
}

func recordPendingRevision(local *state.AgentLocalState, revisionID string, now time.Time) {
	local.RevisionCache.PendingRevisionID = revisionID
	local.RevisionCache.UpdatedAt = now
}
