package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"lazyops-agent/internal/contracts"
)

type FilesystemDriver struct {
	logger *slog.Logger
	root   string
	now    func() time.Time
}

func NewFilesystemDriver(logger *slog.Logger, root string) *FilesystemDriver {
	return &FilesystemDriver{
		logger: logger,
		root:   root,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (d *FilesystemDriver) PrepareReleaseWorkspace(_ context.Context, runtimeCtx RuntimeContext) (PreparedWorkspace, error) {
	if runtimeCtx.Binding.RuntimeMode == contracts.RuntimeModeDistributedK3s {
		return PreparedWorkspace{}, fmt.Errorf("filesystem runtime driver does not support %q", runtimeCtx.Binding.RuntimeMode)
	}
	if filepath.Clean(d.root) == "." {
		return PreparedWorkspace{}, fmt.Errorf("runtime root must be configured")
	}

	layout := workspaceLayout(d.root, runtimeCtx)
	for _, path := range []string{
		layout.Root,
		layout.Artifacts,
		layout.Config,
		layout.Sidecars,
		layout.Gateway,
		layout.Services,
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return PreparedWorkspace{}, err
		}
	}

	serviceManifests := make(map[string]string, len(runtimeCtx.Services))
	for _, service := range runtimeCtx.Services {
		serviceDir := filepath.Join(layout.Services, service.Name)
		if err := os.MkdirAll(serviceDir, 0o755); err != nil {
			return PreparedWorkspace{}, err
		}
		serviceManifestPath := filepath.Join(serviceDir, "service.json")
		if err := writeJSON(serviceManifestPath, service); err != nil {
			return PreparedWorkspace{}, err
		}
		serviceManifests[service.Name] = serviceManifestPath
	}

	if err := writeJSON(filepath.Join(layout.Config, "project.json"), runtimeCtx.Project); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Config, "binding.json"), runtimeCtx.Binding); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Config, "revision.json"), runtimeCtx.Revision); err != nil {
		return PreparedWorkspace{}, err
	}

	artifactPlan := ArtifactPlan{
		Status:     "pending_fetch",
		RevisionID: runtimeCtx.Revision.RevisionID,
	}
	if err := writeJSON(filepath.Join(layout.Artifacts, "manifest.json"), artifactPlan); err != nil {
		return PreparedWorkspace{}, err
	}

	sidecarPlan := SidecarPlan{
		EnabledServices: serviceNames(runtimeCtx.Services),
		Compatibility:   runtimeCtx.Revision.CompatibilityPolicy,
	}
	if err := writeJSON(filepath.Join(layout.Sidecars, "plan.json"), sidecarPlan); err != nil {
		return PreparedWorkspace{}, err
	}

	gatewayPlan := GatewayPlan{
		Provider:       "caddy",
		PublicServices: publicServiceNames(runtimeCtx.Services),
		MagicDomain:    runtimeCtx.Revision.MagicDomainPolicy.Provider,
	}
	if err := writeJSON(filepath.Join(layout.Gateway, "plan.json"), gatewayPlan); err != nil {
		return PreparedWorkspace{}, err
	}

	manifest := WorkspaceManifest{
		PreparedAt:   d.now(),
		Project:      runtimeCtx.Project,
		Binding:      runtimeCtx.Binding,
		Revision:     runtimeCtx.Revision,
		Services:     runtimeCtx.Services,
		Layout:       layout,
		ArtifactPlan: artifactPlan,
		GatewayPlan:  gatewayPlan,
		SidecarPlan:  sidecarPlan,
	}
	if err := capabilityNoContainerLeak(manifest); err != nil {
		return PreparedWorkspace{}, err
	}

	manifestPath := filepath.Join(layout.Root, "workspace.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return PreparedWorkspace{}, err
	}

	if d.logger != nil {
		d.logger.Info("prepared release workspace",
			"project_id", runtimeCtx.Project.ProjectID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"workspace_root", layout.Root,
			"services", len(runtimeCtx.Services),
		)
	}

	return PreparedWorkspace{
		Layout:           layout,
		ManifestPath:     manifestPath,
		ServiceManifests: serviceManifests,
	}, nil
}

func (d *FilesystemDriver) StartReleaseCandidate(context.Context, RuntimeContext) error {
	return ErrNotImplemented
}

func (d *FilesystemDriver) RunHealthGate(context.Context, RuntimeContext) error {
	return ErrNotImplemented
}

func (d *FilesystemDriver) PromoteRelease(context.Context, RuntimeContext) error {
	return ErrNotImplemented
}

func (d *FilesystemDriver) RollbackRelease(context.Context, RuntimeContext) error {
	return ErrNotImplemented
}

func (d *FilesystemDriver) SleepService(context.Context, RuntimeContext, string) error {
	return ErrNotImplemented
}

func (d *FilesystemDriver) WakeService(context.Context, RuntimeContext, string) error {
	return ErrNotImplemented
}

func (d *FilesystemDriver) GarbageCollectRuntime(context.Context, RuntimeContext) error {
	return ErrNotImplemented
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func serviceNames(services []ServiceRuntimeContext) []string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		names = append(names, service.Name)
	}
	sort.Strings(names)
	return names
}

func publicServiceNames(services []ServiceRuntimeContext) []string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		if service.Public {
			names = append(names, service.Name)
		}
	}
	sort.Strings(names)
	return names
}
