package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/dispatcher"
	"lazyops-agent/internal/state"
)

func TestContextFromPreparePayloadRejectsK3s(t *testing.T) {
	_, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeDistributedK3s))
	if err == nil {
		t.Fatal("expected k3s runtime mode to be rejected by local runtime driver context")
	}
}

func TestContextFromPreparePayloadInjectsPlacementRuntimeView(t *testing.T) {
	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if runtimeCtx.Runtime.PlacementFingerprint == "" {
		t.Fatal("expected runtime context to include placement fingerprint")
	}
	if len(runtimeCtx.Runtime.ServiceByName) != 2 {
		t.Fatalf("expected runtime service lookup to include 2 services, got %d", len(runtimeCtx.Runtime.ServiceByName))
	}
	if got := runtimeCtx.Runtime.PlacementByService["api"].TargetID; got != "mesh_remote" {
		t.Fatalf("expected api placement target mesh_remote, got %q", got)
	}
	if got := runtimeCtx.Runtime.ServiceByName["web"].Placement.TargetID; got != "mesh_local" {
		t.Fatalf("expected web service runtime placement mesh_local, got %q", got)
	}
}

func TestFilesystemDriverPrepareReleaseWorkspaceCreatesLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	}

	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}

	prepared, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	for _, path := range []string{
		prepared.Layout.Root,
		prepared.Layout.Artifacts,
		prepared.Layout.Config,
		prepared.Layout.Sidecars,
		prepared.Layout.Gateway,
		prepared.Layout.Mesh,
		prepared.Layout.Services,
		prepared.ManifestPath,
		prepared.ServiceManifests["api"],
		prepared.ServiceManifests["web"],
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected path %s to exist: %v", path, err)
		}
	}

	manifestRaw, err := os.ReadFile(prepared.ManifestPath)
	if err != nil {
		t.Fatalf("read workspace manifest: %v", err)
	}
	if strings.Contains(strings.ToLower(string(manifestRaw)), "container") {
		t.Fatal("expected workspace manifest to remain service-oriented and not leak container terminology")
	}

	var manifest WorkspaceManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode workspace manifest: %v", err)
	}
	if manifest.ArtifactPlan.Status != "cached" {
		t.Fatalf("expected artifact plan status cached, got %q", manifest.ArtifactPlan.Status)
	}
	if got := len(manifest.GatewayPlan.PublicServices); got != 1 {
		t.Fatalf("expected 1 public service in gateway plan, got %d", got)
	}
	if manifest.ArtifactPlan.CacheKey == "" || manifest.ArtifactPlan.CachePath == "" {
		t.Fatal("expected hydrated artifact plan to include cache metadata")
	}
	if _, err := os.Stat(filepath.Join(root, "cache", "assets", manifest.ArtifactPlan.CacheKey, "cache-manifest.json")); err != nil {
		t.Fatalf("expected cache manifest to exist: %v", err)
	}
	if _, err := os.Stat(prepared.SidecarConfigPath); err != nil {
		t.Fatalf("expected sidecar config to exist: %v", err)
	}
	if _, err := os.Stat(prepared.GatewayConfigPath); err != nil {
		t.Fatalf("expected gateway config to exist: %v", err)
	}
	if prepared.MeshStatePath != "" || prepared.ServiceCachePath != "" {
		t.Fatalf("expected standalone prepare release workspace to keep mesh foundation disabled, got mesh paths %#v", prepared)
	}
}

func TestFilesystemDriverPrepareReleaseWorkspaceBuildsMeshFoundationInDistributedMesh(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 11, 30, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}

	prepared, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if prepared.MeshStatePath == "" || prepared.ServiceCachePath == "" {
		t.Fatalf("expected distributed mesh prepare release workspace to record mesh foundation paths, got %#v", prepared)
	}
	for _, path := range []string{prepared.MeshStatePath, prepared.ServiceCachePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected mesh artifact %s to exist: %v", path, err)
		}
	}

	var snapshot MeshFoundationSnapshot
	meshRaw, err := os.ReadFile(prepared.MeshStatePath)
	if err != nil {
		t.Fatalf("read mesh snapshot: %v", err)
	}
	if err := json.Unmarshal(meshRaw, &snapshot); err != nil {
		t.Fatalf("decode mesh snapshot: %v", err)
	}
	if !snapshot.Enabled {
		t.Fatal("expected distributed mesh snapshot to be enabled")
	}
	if snapshot.Health.Status != "planning" {
		t.Fatalf("expected planning mesh health while remote peers are still planned, got %q", snapshot.Health.Status)
	}
	if len(snapshot.Membership.Peers) != 2 {
		t.Fatalf("expected 2 mesh peers, got %d", len(snapshot.Membership.Peers))
	}
	if snapshot.Membership.LocalState != MeshPeerStateActive {
		t.Fatalf("expected local mesh peer state active, got %q", snapshot.Membership.LocalState)
	}
	if len(snapshot.RouteCache) != 1 {
		t.Fatalf("expected 1 mesh route, got %d", len(snapshot.RouteCache))
	}
	if snapshot.RouteCache[0].PathKind != "mesh_private" {
		t.Fatalf("expected cross-node dependency to require mesh_private route, got %#v", snapshot.RouteCache[0])
	}

	var cache ServiceMetadataCache
	cacheRaw, err := os.ReadFile(prepared.ServiceCachePath)
	if err != nil {
		t.Fatalf("read mesh service cache: %v", err)
	}
	if err := json.Unmarshal(cacheRaw, &cache); err != nil {
		t.Fatalf("decode mesh service cache: %v", err)
	}
	webMeta, ok := cache.Services["web"]
	if !ok {
		t.Fatal("expected service metadata cache to include web")
	}
	if webMeta.PlacementTargetID != "mesh_remote" {
		t.Fatalf("expected web placement target mesh_remote, got %q", webMeta.PlacementTargetID)
	}
	if len(webMeta.Dependencies) != 1 {
		t.Fatalf("expected web metadata to include 1 dependency, got %#v", webMeta.Dependencies)
	}
	if webMeta.Dependencies[0].RouteScope != "mesh_private" || !webMeta.Dependencies[0].PrivateOnly {
		t.Fatalf("expected web dependency to be routed through mesh_private, got %#v", webMeta.Dependencies[0])
	}

	manifestRaw, err := os.ReadFile(prepared.ManifestPath)
	if err != nil {
		t.Fatalf("read workspace manifest: %v", err)
	}
	var manifest WorkspaceManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode workspace manifest: %v", err)
	}
	if manifest.MeshSnapshot == nil || !manifest.MeshSnapshot.Enabled {
		t.Fatalf("expected workspace manifest to embed enabled mesh snapshot, got %#v", manifest.MeshSnapshot)
	}
}

func TestMeshServiceRegistersMeshCommands(t *testing.T) {
	registry := dispatcher.NewRegistry()
	service := NewMeshService(nil, nil, NewMeshManager(nil, filepath.Join(t.TempDir(), "runtime-root")))
	service.Register(registry)

	if _, ok := registry.Resolve(contracts.CommandEnsureMeshPeer); !ok {
		t.Fatal("expected mesh service to register ensure_mesh_peer handler")
	}
	if _, ok := registry.Resolve(contracts.CommandSyncOverlayRoutes); !ok {
		t.Fatal("expected mesh service to register sync_overlay_routes handler")
	}
}

func TestMeshServiceEnsureMeshPeerCreatesWireGuardStateAndSignals(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	driver := NewFilesystemDriver(logger, root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	manager := NewMeshManager(logger, root)
	manager.now = driver.now
	service := NewMeshService(logger, store, manager)
	service.now = driver.now

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
	}
	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	raw, err := json.Marshal(contracts.EnsureMeshPeerPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
		PeerRef:     "mesh:mesh_remote",
		TargetID:    "mesh_remote",
		TargetKind:  contracts.TargetKindMesh,
	})
	if err != nil {
		t.Fatalf("marshal ensure mesh peer payload: %v", err)
	}

	result := service.handleEnsureMeshPeer(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandEnsureMeshPeer,
		RequestID:     "req_mesh_ensure",
		CorrelationID: "corr_mesh_ensure",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected ensure mesh peer to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	statePath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "state.json")
	snapshot, err := loadMeshFoundationSnapshot(statePath)
	if err != nil {
		t.Fatalf("load mesh live state: %v", err)
	}
	foundActiveRemote := false
	for _, peer := range snapshot.Membership.Peers {
		if peer.PeerRef == "mesh:mesh_remote" {
			if peer.State != MeshPeerStateActive {
				t.Fatalf("expected remote peer to become active, got %q", peer.State)
			}
			foundActiveRemote = true
		}
	}
	if !foundActiveRemote {
		t.Fatal("expected remote mesh peer to exist in live state")
	}
	if snapshot.Health.Status != "active" {
		t.Fatalf("expected mesh health to become active after ensure, got %q", snapshot.Health.Status)
	}

	for _, path := range []string{
		filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "health-signals.json"),
		filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "capability-signals.json"),
		filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "wireguard", sanitizePathToken("mesh:mesh_remote"), "peer.json"),
		filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "wireguard", sanitizePathToken("mesh:mesh_remote"), "wg.conf"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected mesh artifact %s to exist: %v", path, err)
		}
	}

	privateKeyPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "wireguard", sanitizePathToken("mesh:mesh_remote"), "private.key")
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		t.Fatalf("read wireguard private key: %v", err)
	}
	peerRecordRaw, err := os.ReadFile(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "wireguard", sanitizePathToken("mesh:mesh_remote"), "peer.json"))
	if err != nil {
		t.Fatalf("read wireguard peer record: %v", err)
	}
	if strings.Contains(string(peerRecordRaw), strings.TrimSpace(string(privateKey))) {
		t.Fatal("expected peer record to avoid storing plaintext private key")
	}
}

func TestMeshServiceEnsureMeshPeerRemovesPeerDeterministically(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	driver := NewFilesystemDriver(logger, root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 2, 9, 30, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	manager := NewMeshManager(logger, root)
	manager.now = driver.now

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
	}
	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	if _, err := manager.EnsurePeer(context.Background(), contracts.EnsureMeshPeerPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
		PeerRef:     "mesh:mesh_remote",
		TargetID:    "mesh_remote",
		TargetKind:  contracts.TargetKindMesh,
	}); err != nil {
		t.Fatalf("ensure active mesh peer: %v", err)
	}

	result, err := manager.EnsurePeer(context.Background(), contracts.EnsureMeshPeerPayload{
		ProjectID:    payload.Project.ProjectID,
		BindingID:    payload.Binding.BindingID,
		RevisionID:   payload.Revision.RevisionID,
		RuntimeMode:  payload.Binding.RuntimeMode,
		Provider:     contracts.MeshProviderWireGuard,
		PeerRef:      "mesh:mesh_remote",
		TargetID:     "mesh_remote",
		TargetKind:   contracts.TargetKindMesh,
		DesiredState: string(MeshPeerStateRemoved),
	})
	if err != nil {
		t.Fatalf("remove mesh peer: %v", err)
	}
	if len(result.RemovedPeerRefs) != 1 || result.RemovedPeerRefs[0] != "mesh:mesh_remote" {
		t.Fatalf("expected removed peer mesh:mesh_remote, got %#v", result.RemovedPeerRefs)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "wireguard", sanitizePathToken("mesh:mesh_remote"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected wireguard peer directory to be removed, got %v", err)
	}

	snapshot, err := loadMeshFoundationSnapshot(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "state.json"))
	if err != nil {
		t.Fatalf("load mesh live state after removal: %v", err)
	}
	for _, peer := range snapshot.Membership.Peers {
		if peer.PeerRef == "mesh:mesh_remote" && peer.State != MeshPeerStateRemoved {
			t.Fatalf("expected remote peer to become removed, got %q", peer.State)
		}
	}
}

func TestMeshServiceEnsureMeshPeerRejectsStandaloneRuntime(t *testing.T) {
	manager := NewMeshManager(nil, filepath.Join(t.TempDir(), "runtime-root"))
	_, err := manager.EnsurePeer(context.Background(), contracts.EnsureMeshPeerPayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		Provider:    contracts.MeshProviderWireGuard,
	})
	if err == nil {
		t.Fatal("expected ensure mesh peer to reject standalone runtime")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "mesh_runtime_mode_disabled" {
		t.Fatalf("expected mesh_runtime_mode_disabled code, got %q", opErr.Code)
	}
	if opErr.Retryable {
		t.Fatal("expected standalone mesh ensure failure to be non-retryable")
	}
}

func TestMeshServiceSyncOverlayRoutesVerifiesPrivateRoutesAndWritesAdapterSlots(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	driver := NewFilesystemDriver(logger, root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	manager := NewMeshManager(logger, root)
	manager.now = driver.now
	service := NewMeshService(logger, store, manager)
	service.now = driver.now

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
	}
	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := manager.EnsurePeer(context.Background(), contracts.EnsureMeshPeerPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
		PeerRef:     "mesh:mesh_remote",
		TargetID:    "mesh_remote",
		TargetKind:  contracts.TargetKindMesh,
	}); err != nil {
		t.Fatalf("ensure active mesh peer: %v", err)
	}

	raw, err := json.Marshal(contracts.SyncOverlayRoutesPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
	})
	if err != nil {
		t.Fatalf("marshal sync overlay routes payload: %v", err)
	}

	result := service.handleSyncOverlayRoutes(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSyncOverlayRoutes,
		RequestID:     "req_overlay_sync",
		CorrelationID: "corr_overlay_sync",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected sync overlay routes to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	routeReportPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "overlay-routes.json")
	linkHealthPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "link-health.json")
	adapterSlotsPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "adapter-slots.json")

	var routes []MeshRouteRecord
	routesRaw, err := os.ReadFile(routeReportPath)
	if err != nil {
		t.Fatalf("read overlay route report: %v", err)
	}
	if err := json.Unmarshal(routesRaw, &routes); err != nil {
		t.Fatalf("decode overlay route report: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 overlay route, got %d", len(routes))
	}
	if routes[0].Status != "verified" || !routes[0].Verified {
		t.Fatalf("expected private route to be verified, got %#v", routes[0])
	}
	if !routes[0].PublicFallbackBlocked {
		t.Fatalf("expected private route to block public fallback, got %#v", routes[0])
	}
	if routes[0].Provider != contracts.MeshProviderWireGuard {
		t.Fatalf("expected private route provider wireguard, got %q", routes[0].Provider)
	}

	var links []MeshLinkHealthRecord
	linksRaw, err := os.ReadFile(linkHealthPath)
	if err != nil {
		t.Fatalf("read link health report: %v", err)
	}
	if err := json.Unmarshal(linksRaw, &links); err != nil {
		t.Fatalf("decode link health report: %v", err)
	}
	if len(links) != 1 || links[0].Status != "verified" {
		t.Fatalf("expected 1 verified link health record, got %#v", links)
	}

	var adapters []MeshAdapterSlot
	adaptersRaw, err := os.ReadFile(adapterSlotsPath)
	if err != nil {
		t.Fatalf("read adapter slots report: %v", err)
	}
	if err := json.Unmarshal(adaptersRaw, &adapters); err != nil {
		t.Fatalf("decode adapter slots report: %v", err)
	}
	if len(adapters) != 2 {
		t.Fatalf("expected 2 adapter slots, got %d", len(adapters))
	}
	if adapters[0].Provider != contracts.MeshProviderWireGuard || !adapters[0].Active {
		t.Fatalf("expected first adapter slot to be active wireguard, got %#v", adapters[0])
	}
	if adapters[1].Provider != contracts.MeshProviderTailscale || !adapters[1].Reserved || adapters[1].Status != "reserved" {
		t.Fatalf("expected second adapter slot to reserve tailscale, got %#v", adapters[1])
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "tailscale", "slot.json")); err != nil {
		t.Fatalf("expected tailscale reserved slot artifact to exist: %v", err)
	}

	cache, err := loadServiceMetadataCache(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "service-metadata.json"))
	if err != nil {
		t.Fatalf("load synced service metadata cache: %v", err)
	}
	if got := cache.Services["web"].Dependencies[0].RouteStatus; got != "verified" {
		t.Fatalf("expected synced dependency route status verified, got %q", got)
	}
	if !cache.Services["web"].Dependencies[0].PublicFallbackBlocked {
		t.Fatalf("expected synced dependency to block public fallback, got %#v", cache.Services["web"].Dependencies[0])
	}

	snapshot, err := loadMeshFoundationSnapshot(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "mesh", "live", "state.json"))
	if err != nil {
		t.Fatalf("load synced mesh state: %v", err)
	}
	if snapshot.Health.VerifiedRoutes != 1 || snapshot.Health.BlockedRoutes != 0 || snapshot.Health.Status != "active" {
		t.Fatalf("expected active mesh health with 1 verified route, got %#v", snapshot.Health)
	}
}

func TestMeshServiceSyncOverlayRoutesBlocksPrivateRouteWithoutActivePeer(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	driver := NewFilesystemDriver(logger, root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 2, 10, 30, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	manager := NewMeshManager(logger, root)
	manager.now = driver.now

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
	}
	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	result, err := manager.SyncOverlayRoutes(context.Background(), contracts.SyncOverlayRoutesPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
	})
	if err != nil {
		t.Fatalf("sync overlay routes: %v", err)
	}
	if result.BlockedRoutes != 1 || result.VerifiedRoutes != 0 {
		t.Fatalf("expected 1 blocked route and 0 verified routes, got %#v", result)
	}

	var routes []MeshRouteRecord
	routesRaw, err := os.ReadFile(result.RouteReportPath)
	if err != nil {
		t.Fatalf("read blocked route report: %v", err)
	}
	if err := json.Unmarshal(routesRaw, &routes); err != nil {
		t.Fatalf("decode blocked route report: %v", err)
	}
	if len(routes) != 1 || routes[0].Status != "blocked" {
		t.Fatalf("expected route to be blocked without active peer, got %#v", routes)
	}
	if !routes[0].PublicFallbackBlocked {
		t.Fatalf("expected blocked private route to keep public fallback blocked, got %#v", routes[0])
	}

	var links []MeshLinkHealthRecord
	linksRaw, err := os.ReadFile(result.LinkHealthPath)
	if err != nil {
		t.Fatalf("read blocked link health report: %v", err)
	}
	if err := json.Unmarshal(linksRaw, &links); err != nil {
		t.Fatalf("decode blocked link health report: %v", err)
	}
	if len(links) != 1 || links[0].Status != "unavailable" {
		t.Fatalf("expected link health to show unavailable, got %#v", links)
	}

	snapshot, err := loadMeshFoundationSnapshot(result.StatePath)
	if err != nil {
		t.Fatalf("load blocked mesh state: %v", err)
	}
	if snapshot.Health.Status != "degraded" || snapshot.Health.BlockedRoutes != 1 {
		t.Fatalf("expected degraded mesh health with blocked route, got %#v", snapshot.Health)
	}
}

func TestMeshServiceSyncOverlayRoutesRejectsStandaloneRuntime(t *testing.T) {
	manager := NewMeshManager(nil, filepath.Join(t.TempDir(), "runtime-root"))
	_, err := manager.SyncOverlayRoutes(context.Background(), contracts.SyncOverlayRoutesPayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		Provider:    contracts.MeshProviderWireGuard,
	})
	if err == nil {
		t.Fatal("expected sync overlay routes to reject standalone runtime")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "mesh_runtime_mode_disabled" {
		t.Fatalf("expected mesh_runtime_mode_disabled code, got %q", opErr.Code)
	}
	if opErr.Retryable {
		t.Fatal("expected standalone overlay sync failure to be non-retryable")
	}
}

func TestServicePrepareReleaseWorkspaceHandlerUpdatesRevisionCache(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), filepath.Join(t.TempDir(), "runtime-root"))
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)
	service.now = func() time.Time {
		return time.Date(2026, 3, 31, 10, 30, 0, 0, time.UTC)
	}

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_prepare",
		CorrelationID: "corr_prepare",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected handler to succeed, got error %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.PendingRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected pending revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.PendingRevisionID)
	}
}

func TestFilesystemDriverStartReleaseCandidateWritesSkeleton(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	candidate, err := driver.StartReleaseCandidate(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("start release candidate: %v", err)
	}
	if candidate.State != "starting" {
		t.Fatalf("expected candidate state starting, got %q", candidate.State)
	}
	if len(candidate.History) != 2 {
		t.Fatalf("expected prepared->starting history, got %d transitions", len(candidate.History))
	}
	if candidate.History[0].To != CandidateStatePrepared || candidate.History[1].To != CandidateStateStarting {
		t.Fatalf("expected candidate history to include prepared and starting, got %#v", candidate.History)
	}
	if _, err := os.Stat(candidate.ManifestPath); err != nil {
		t.Fatalf("expected candidate manifest to exist: %v", err)
	}
	if !strings.HasPrefix(candidate.WorkspaceRoot, root) {
		t.Fatalf("expected candidate workspace root under %s, got %s", root, candidate.WorkspaceRoot)
	}
}

func TestFilesystemDriverRenderGatewayConfigCreatesVersionedPlanAndLiveConfig(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	}
	if driver.gateway != nil {
		driver.gateway.now = driver.now
	}

	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderGatewayConfig(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render gateway config: %v", err)
	}
	if rendered.Version == "" {
		t.Fatal("expected gateway version to be set")
	}
	if len(rendered.PublicURLs) != 2 {
		t.Fatalf("expected 2 public urls for one public service, got %d", len(rendered.PublicURLs))
	}
	if !strings.Contains(rendered.PublicURLs[0], ".sslip.io") {
		t.Fatalf("expected primary public url to use sslip.io, got %q", rendered.PublicURLs[0])
	}
	if !strings.Contains(rendered.PublicURLs[1], ".nip.io") {
		t.Fatalf("expected fallback public url to use nip.io, got %q", rendered.PublicURLs[1])
	}
	if _, err := os.Stat(rendered.PlanPath); err != nil {
		t.Fatalf("expected versioned gateway plan to exist: %v", err)
	}
	if _, err := os.Stat(rendered.ConfigPath); err != nil {
		t.Fatalf("expected versioned Caddyfile to exist: %v", err)
	}
	if _, err := os.Stat(rendered.LiveConfigPath); err != nil {
		t.Fatalf("expected live Caddyfile to exist: %v", err)
	}

	liveConfigRaw, err := os.ReadFile(rendered.LiveConfigPath)
	if err != nil {
		t.Fatalf("read live Caddyfile: %v", err)
	}
	if strings.Contains(string(liveConfigRaw), "127.0.0.1:8080") {
		t.Fatal("expected internal api service port to remain private and absent from public gateway config")
	}
	if !strings.Contains(string(liveConfigRaw), "127.0.0.1:3000") {
		t.Fatal("expected public web service upstream to be present in gateway config")
	}

	planRaw, err := os.ReadFile(rendered.LivePlanPath)
	if err != nil {
		t.Fatalf("read live gateway plan: %v", err)
	}
	var plan GatewayPlan
	if err := json.Unmarshal(planRaw, &plan); err != nil {
		t.Fatalf("decode live gateway plan: %v", err)
	}
	if plan.MagicDomain != "sslip.io" || plan.FallbackMagicDomain != "nip.io" {
		t.Fatalf("expected sslip/nip magic domain order, got %q -> %q", plan.MagicDomain, plan.FallbackMagicDomain)
	}
	if len(plan.Routes) != 1 || plan.Routes[0].ServiceName != "web" {
		t.Fatalf("expected exactly one public route for web, got %#v", plan.Routes)
	}
	if plan.Validation == nil || plan.Apply == nil || plan.Reload == nil {
		t.Fatalf("expected validate/apply/reload hook results to be recorded, got %#v", plan)
	}
}

func TestFilesystemDriverRenderGatewayConfigRollsBackOnReloadFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	firstPayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	firstCtx, err := ContextFromPreparePayload(firstPayload)
	if err != nil {
		t.Fatalf("build first runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), firstCtx); err != nil {
		t.Fatalf("prepare first release workspace: %v", err)
	}
	firstRendered, err := driver.RenderGatewayConfig(context.Background(), firstCtx)
	if err != nil {
		t.Fatalf("render first gateway config: %v", err)
	}

	secondPayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	secondPayload.Revision.RevisionID = "rev_124"
	secondCtx, err := ContextFromPreparePayload(secondPayload)
	if err != nil {
		t.Fatalf("build second runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), secondCtx); err != nil {
		t.Fatalf("prepare second release workspace: %v", err)
	}

	driver.gateway.reloadHook = func(context.Context, GatewayPlan, gatewayRenderPaths, GatewayActivation) (GatewayHookResult, error) {
		return GatewayHookResult{}, &OperationError{
			Code:      "gateway_reload_timeout",
			Message:   "gateway reload timed out",
			Retryable: true,
		}
	}

	_, err = driver.RenderGatewayConfig(context.Background(), secondCtx)
	if err == nil {
		t.Fatal("expected second gateway render to fail on reload")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "gateway_reload_failed" {
		t.Fatalf("expected gateway_reload_failed code, got %q", opErr.Code)
	}

	active, err := loadGatewayActivation(filepath.Join(root, "projects", secondCtx.Project.ProjectID, "bindings", secondCtx.Binding.BindingID, "gateway", "live", "active.json"))
	if err != nil {
		t.Fatalf("load active gateway activation after rollback: %v", err)
	}
	if active.Version != firstRendered.Version {
		t.Fatalf("expected rollback to restore first active version %q, got %q", firstRendered.Version, active.Version)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", secondCtx.Project.ProjectID, "bindings", secondCtx.Binding.BindingID, "gateway", "live", "rollback.json")); err != nil {
		t.Fatalf("expected rollback manifest to exist: %v", err)
	}
}

func TestFilesystemDriverRenderGatewayConfigUsesMeshPlacementResolutionForRemotePublicService(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 2, 12, 30, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	if driver.gateway != nil {
		driver.gateway.now = driver.now
	}

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := driver.mesh.EnsurePeer(context.Background(), contracts.EnsureMeshPeerPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
		PeerRef:     "mesh:mesh_remote",
		TargetID:    "mesh_remote",
		TargetKind:  contracts.TargetKindMesh,
	}); err != nil {
		t.Fatalf("ensure remote peer: %v", err)
	}

	rendered, err := driver.RenderGatewayConfig(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render gateway config: %v", err)
	}
	if len(rendered.Plan.Routes) != 1 {
		t.Fatalf("expected one public route, got %#v", rendered.Plan.Routes)
	}
	route := rendered.Plan.Routes[0]
	if route.RouteScope != "mesh_private" {
		t.Fatalf("expected mesh_private gateway route scope, got %#v", route)
	}
	if route.ResolutionStatus != "verified" {
		t.Fatalf("expected verified gateway resolution, got %#v", route)
	}
	if route.PlacementPeerRef != "mesh:mesh_remote" {
		t.Fatalf("expected remote placement peer mesh:mesh_remote, got %#v", route)
	}
	if !route.PublicFallbackBlocked {
		t.Fatalf("expected remote gateway route to keep public fallback blocked, got %#v", route)
	}
	if strings.Contains(route.Upstream, "127.0.0.1") || !strings.Contains(route.Upstream, ".mesh.lazyops.internal:3000") {
		t.Fatalf("expected remote gateway upstream to use mesh host, got %#v", route)
	}
	if !containsString(rendered.Plan.InvalidationRules, "mesh_peer_health_changed") {
		t.Fatalf("expected gateway plan to expose invalidation rules, got %#v", rendered.Plan.InvalidationRules)
	}

	liveConfigRaw, err := os.ReadFile(rendered.LiveConfigPath)
	if err != nil {
		t.Fatalf("read live Caddyfile: %v", err)
	}
	if !strings.Contains(string(liveConfigRaw), ".mesh.lazyops.internal:3000") {
		t.Fatalf("expected live Caddyfile to use mesh upstream, got %q", string(liveConfigRaw))
	}
}

func TestFilesystemDriverRenderSidecarsInjectsRuntimeConfigAndMetadataCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 9, 30, 0, 0, time.UTC)
	}
	if driver.sidecar != nil {
		driver.sidecar.now = driver.now
	}

	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	if rendered.Version == "" {
		t.Fatal("expected sidecar version to be set")
	}
	if len(rendered.Services) != 1 || rendered.Services[0] != "web" {
		t.Fatalf("expected only web to have sidecar bindings, got %#v", rendered.Services)
	}
	if _, err := os.Stat(rendered.MetadataCachePath); err != nil {
		t.Fatalf("expected metadata cache to exist: %v", err)
	}

	var plan SidecarPlan
	planRaw, err := os.ReadFile(rendered.PlanPath)
	if err != nil {
		t.Fatalf("read sidecar plan: %v", err)
	}
	if err := json.Unmarshal(planRaw, &plan); err != nil {
		t.Fatalf("decode sidecar plan: %v", err)
	}
	if plan.Services[0].SelectedMode != "env_injection" {
		t.Fatalf("expected env_injection mode, got %q", plan.Services[0].SelectedMode)
	}
	if plan.Services[0].Env["LAZYOPS_DEP_API_ENDPOINT"] != "http://localhost:8080" {
		t.Fatalf("expected env injection endpoint for api dependency, got %#v", plan.Services[0].Env)
	}

	webRuntimePath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "revisions", "rev_123", "services", "web", "runtime.json")
	var webRuntime map[string]any
	webRaw, err := os.ReadFile(webRuntimePath)
	if err != nil {
		t.Fatalf("read web runtime config: %v", err)
	}
	if err := json.Unmarshal(webRaw, &webRuntime); err != nil {
		t.Fatalf("decode web runtime config: %v", err)
	}
	sidecarBlock, ok := webRuntime["sidecar"].(map[string]any)
	if !ok {
		t.Fatalf("expected web runtime sidecar block, got %#v", webRuntime["sidecar"])
	}
	if enabled, _ := sidecarBlock["enabled"].(bool); !enabled {
		t.Fatal("expected web runtime to have sidecar enabled")
	}
	if sidecarBlock["selected_mode"] != "env_injection" {
		t.Fatalf("expected web runtime selected_mode env_injection, got %#v", sidecarBlock["selected_mode"])
	}

	apiRuntimePath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "revisions", "rev_123", "services", "api", "runtime.json")
	var apiRuntime map[string]any
	apiRaw, err := os.ReadFile(apiRuntimePath)
	if err != nil {
		t.Fatalf("read api runtime config: %v", err)
	}
	if err := json.Unmarshal(apiRaw, &apiRuntime); err != nil {
		t.Fatalf("decode api runtime config: %v", err)
	}
	apiSidecarBlock, ok := apiRuntime["sidecar"].(map[string]any)
	if !ok {
		t.Fatalf("expected api runtime sidecar block, got %#v", apiRuntime["sidecar"])
	}
	if enabled, _ := apiSidecarBlock["enabled"].(bool); enabled {
		t.Fatal("expected api runtime to keep sidecar disabled without dependencies")
	}

	var metadata SidecarMetadataCache
	metadataRaw, err := os.ReadFile(rendered.MetadataCachePath)
	if err != nil {
		t.Fatalf("read sidecar metadata cache: %v", err)
	}
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		t.Fatalf("decode sidecar metadata cache: %v", err)
	}
	webMeta, ok := metadata.Services["web"]
	if !ok {
		t.Fatal("expected metadata cache to include web")
	}
	if webMeta.SelectedMode != "env_injection" {
		t.Fatalf("expected metadata selected mode env_injection, got %q", webMeta.SelectedMode)
	}
	if webMeta.ConfigPath == "" || webMeta.RuntimePath == "" {
		t.Fatalf("expected metadata cache to include config/runtime paths, got %#v", webMeta)
	}
}

func TestFilesystemDriverRenderSidecarsBuildsValidatedEnvContracts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	if len(rendered.Plan.Services) != 1 {
		t.Fatalf("expected one sidecar service config, got %#v", rendered.Plan.Services)
	}
	if rendered.Plan.Services[0].SelectedMode != "env_injection" {
		t.Fatalf("expected env_injection mode, got %#v", rendered.Plan.Services[0])
	}
	if len(rendered.Plan.Services[0].EnvContracts) != 1 {
		t.Fatalf("expected one env contract, got %#v", rendered.Plan.Services[0].EnvContracts)
	}
	contract := rendered.Plan.Services[0].EnvContracts[0]
	if !contract.SecretSafe {
		t.Fatalf("expected env contract to remain secret safe, got %#v", contract)
	}
	for _, key := range contract.RequiredKeys {
		if strings.TrimSpace(contract.Values[key]) == "" {
			t.Fatalf("expected env contract key %q to be populated, got %#v", key, contract.Values)
		}
	}
	if contract.Values["LAZYOPS_DEP_API_ENDPOINT"] != "http://localhost:8080" {
		t.Fatalf("expected env contract endpoint http://localhost:8080, got %#v", contract.Values)
	}

	webRuntimePath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "revisions", "rev_123", "services", "web", "runtime.json")
	webRaw, err := os.ReadFile(webRuntimePath)
	if err != nil {
		t.Fatalf("read web runtime config: %v", err)
	}
	var webRuntime map[string]any
	if err := json.Unmarshal(webRaw, &webRuntime); err != nil {
		t.Fatalf("decode web runtime config: %v", err)
	}
	sidecarBlock, ok := webRuntime["sidecar"].(map[string]any)
	if !ok {
		t.Fatalf("expected sidecar runtime block, got %#v", webRuntime["sidecar"])
	}
	envContracts, ok := sidecarBlock["env_contracts"].([]any)
	if !ok || len(envContracts) != 1 {
		t.Fatalf("expected runtime sidecar block to include one env contract, got %#v", sidecarBlock["env_contracts"])
	}
}

func TestFilesystemDriverRenderSidecarsUsesManagedCredentialsFallbackWhenEnvInjectionDisabled(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	payload.Binding.CompatibilityPolicy.EnvInjection = false
	payload.Binding.CompatibilityPolicy.ManagedCredentials = true
	payload.Revision.CompatibilityPolicy.EnvInjection = false
	payload.Revision.CompatibilityPolicy.ManagedCredentials = true

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	if rendered.Plan.Services[0].SelectedMode != "managed_credentials" {
		t.Fatalf("expected managed_credentials mode, got %q", rendered.Plan.Services[0].SelectedMode)
	}
	if len(rendered.Plan.Services[0].ManagedCredentials) == 0 {
		t.Fatal("expected managed credential injection config to be generated")
	}
	if len(rendered.Plan.Services[0].ManagedCredentialContracts) != 1 {
		t.Fatalf("expected one managed credential contract, got %#v", rendered.Plan.Services[0].ManagedCredentialContracts)
	}
	if len(rendered.Plan.Services[0].EnvContracts) != 0 {
		t.Fatalf("expected env contracts to stay empty in managed credential mode, got %#v", rendered.Plan.Services[0].EnvContracts)
	}
	if len(rendered.Plan.Services[0].ProxyRoutes) != 0 {
		t.Fatalf("expected localhost rescue to stay disabled after managed credential injection, got %#v", rendered.Plan.Services[0].ProxyRoutes)
	}
	contract := rendered.Plan.Services[0].ManagedCredentialContracts[0]
	if !contract.SecretSafe || !contract.LocalhostRescueSkipped {
		t.Fatalf("expected managed credential contract to be secret-safe and skip localhost rescue, got %#v", contract)
	}
	if contract.CredentialRef != "managed://prj_123/web/api" {
		t.Fatalf("expected managed credential ref managed://prj_123/web/api, got %#v", contract)
	}
	if !strings.HasPrefix(contract.Values["LAZYOPS_MANAGED_API_REF"], "managed://") {
		t.Fatalf("expected managed credential ref env to use managed:// scheme, got %#v", contract.Values)
	}
	if !strings.HasPrefix(contract.Values["LAZYOPS_MANAGED_API_HANDLE"], "mcred_") {
		t.Fatalf("expected managed credential handle env to use mcred_ prefix, got %#v", contract.Values)
	}

	if _, err := os.Stat(rendered.ManagedCredentialAuditPath); err != nil {
		t.Fatalf("expected managed credential audit log to exist: %v", err)
	}
	var audit ManagedCredentialAuditLog
	auditRaw, err := os.ReadFile(rendered.ManagedCredentialAuditPath)
	if err != nil {
		t.Fatalf("read managed credential audit log: %v", err)
	}
	if err := json.Unmarshal(auditRaw, &audit); err != nil {
		t.Fatalf("decode managed credential audit log: %v", err)
	}
	if audit.PlaintextPersisted {
		t.Fatalf("expected audit to record no plaintext persistence, got %#v", audit)
	}
	if len(audit.Services["web"]) != 1 {
		t.Fatalf("expected one managed credential audit record for web, got %#v", audit.Services)
	}
	record := audit.Services["web"][0]
	if record.MaskedValues["LAZYOPS_MANAGED_API_HANDLE"] == contract.Values["LAZYOPS_MANAGED_API_HANDLE"] {
		t.Fatalf("expected audit to mask handle value, got %#v", record)
	}
	if record.ValueFingerprints["LAZYOPS_MANAGED_API_HANDLE"] == "" {
		t.Fatalf("expected audit to store handle fingerprint, got %#v", record)
	}
}

func TestFilesystemDriverRenderSidecarsUsesLocalhostRescueWhenHigherModesDisabled(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	payload.Binding.CompatibilityPolicy.EnvInjection = false
	payload.Binding.CompatibilityPolicy.ManagedCredentials = false
	payload.Binding.CompatibilityPolicy.LocalhostRescue = true
	payload.Revision.CompatibilityPolicy.EnvInjection = false
	payload.Revision.CompatibilityPolicy.ManagedCredentials = false
	payload.Revision.CompatibilityPolicy.LocalhostRescue = true

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	if rendered.Plan.Services[0].SelectedMode != "localhost_rescue" {
		t.Fatalf("expected localhost_rescue mode, got %q", rendered.Plan.Services[0].SelectedMode)
	}
	if len(rendered.Plan.Services[0].LocalhostRescueContracts) != 1 {
		t.Fatalf("expected one localhost rescue contract, got %#v", rendered.Plan.Services[0].LocalhostRescueContracts)
	}
	if len(rendered.Plan.Services[0].ProxyRoutes) != 1 {
		t.Fatalf("expected one proxy route for localhost rescue, got %#v", rendered.Plan.Services[0].ProxyRoutes)
	}
	contract := rendered.Plan.Services[0].LocalhostRescueContracts[0]
	if contract.Protocol != "http" {
		t.Fatalf("expected http localhost rescue contract, got %#v", contract)
	}
	if contract.ListenerEndpoint != "http://localhost:8080" || contract.ListenerHost != "localhost" || contract.ListenerPort != 8080 {
		t.Fatalf("expected localhost http listener to stay on port 8080, got %#v", contract)
	}
	if contract.ForwardingMode != "local_target" {
		t.Fatalf("expected local_target forwarding mode, got %#v", contract)
	}
	if contract.FallbackClass != "service_down" || contract.MeshHealthRequired {
		t.Fatalf("expected standalone localhost rescue to classify failures as service_down without mesh health, got %#v", contract)
	}
	if !contract.NetworkNamespaceIntercept {
		t.Fatalf("expected localhost rescue to stay in the same network namespace, got %#v", contract)
	}

	webRuntimePath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "revisions", "rev_123", "services", "web", "runtime.json")
	webRaw, err := os.ReadFile(webRuntimePath)
	if err != nil {
		t.Fatalf("read web runtime config: %v", err)
	}
	var webRuntime map[string]any
	if err := json.Unmarshal(webRaw, &webRuntime); err != nil {
		t.Fatalf("decode web runtime config: %v", err)
	}
	sidecarBlock, ok := webRuntime["sidecar"].(map[string]any)
	if !ok {
		t.Fatalf("expected sidecar runtime block, got %#v", webRuntime["sidecar"])
	}
	rescueContracts, ok := sidecarBlock["localhost_rescue_contracts"].([]any)
	if !ok || len(rescueContracts) != 1 {
		t.Fatalf("expected runtime sidecar block to include one localhost rescue contract, got %#v", sidecarBlock["localhost_rescue_contracts"])
	}
}

func TestFilesystemDriverRenderSidecarsSupportsTCPLocalhostRescue(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	payload.Binding.CompatibilityPolicy.EnvInjection = false
	payload.Binding.CompatibilityPolicy.ManagedCredentials = false
	payload.Binding.CompatibilityPolicy.LocalhostRescue = true
	payload.Revision.CompatibilityPolicy.EnvInjection = false
	payload.Revision.CompatibilityPolicy.ManagedCredentials = false
	payload.Revision.CompatibilityPolicy.LocalhostRescue = true
	payload.Revision.Services[0].HealthCheck = contracts.HealthCheckPayload{
		Protocol: "tcp",
		Port:     5432,
	}
	payload.Revision.DependencyBindings[0].Protocol = "tcp"
	payload.Revision.DependencyBindings[0].LocalEndpoint = "localhost:5432"

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	contract := rendered.Plan.Services[0].LocalhostRescueContracts[0]
	if contract.Protocol != "tcp" || contract.ListenerScheme != "tcp" {
		t.Fatalf("expected tcp localhost rescue contract, got %#v", contract)
	}
	if contract.ListenerEndpoint != "localhost:5432" || contract.ListenerPort != 5432 {
		t.Fatalf("expected tcp localhost listener to stay on port 5432, got %#v", contract)
	}
	if contract.Upstream != "api.service.lazyops.internal" {
		t.Fatalf("expected tcp upstream without http scheme, got %#v", contract)
	}
}

func TestFilesystemDriverRenderSidecarsUsesMeshForwardingForRemoteLocalhostRescue(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	if driver.sidecar != nil {
		driver.sidecar.now = driver.now
	}

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Binding.CompatibilityPolicy.EnvInjection = false
	payload.Binding.CompatibilityPolicy.ManagedCredentials = false
	payload.Binding.CompatibilityPolicy.LocalhostRescue = true
	payload.Revision.CompatibilityPolicy.EnvInjection = false
	payload.Revision.CompatibilityPolicy.ManagedCredentials = false
	payload.Revision.CompatibilityPolicy.LocalhostRescue = true
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := driver.mesh.EnsurePeer(context.Background(), contracts.EnsureMeshPeerPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
		PeerRef:     "mesh:mesh_remote",
		TargetID:    "mesh_remote",
		TargetKind:  contracts.TargetKindMesh,
	}); err != nil {
		t.Fatalf("ensure remote peer: %v", err)
	}
	if _, err := driver.mesh.SyncOverlayRoutes(context.Background(), contracts.SyncOverlayRoutesPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
	}); err != nil {
		t.Fatalf("sync overlay routes: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	contract := rendered.Plan.Services[0].LocalhostRescueContracts[0]
	if contract.ForwardingMode != "mesh_target" {
		t.Fatalf("expected localhost rescue to forward through mesh_target, got %#v", contract)
	}
	if contract.FallbackClass != "service_down" || !contract.MeshHealthRequired {
		t.Fatalf("expected verified remote rescue to require mesh health but classify remaining failures as service_down, got %#v", contract)
	}
	if contract.PlacementPeerRef != "mesh:mesh_remote" || contract.Provider != contracts.MeshProviderWireGuard {
		t.Fatalf("expected localhost rescue to target the remote wireguard peer, got %#v", contract)
	}
	if !strings.Contains(contract.Upstream, ".mesh.lazyops.internal") {
		t.Fatalf("expected localhost rescue upstream to use the mesh host, got %#v", contract)
	}
}

func TestFilesystemDriverRenderSidecarsClassifiesUnhealthyMeshLocalhostRescueAsNetworkDown(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Binding.CompatibilityPolicy.EnvInjection = false
	payload.Binding.CompatibilityPolicy.ManagedCredentials = false
	payload.Binding.CompatibilityPolicy.LocalhostRescue = true
	payload.Revision.CompatibilityPolicy.EnvInjection = false
	payload.Revision.CompatibilityPolicy.ManagedCredentials = false
	payload.Revision.CompatibilityPolicy.LocalhostRescue = true
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	contract := rendered.Plan.Services[0].LocalhostRescueContracts[0]
	if contract.ForwardingMode != "mesh_target" {
		t.Fatalf("expected localhost rescue to keep mesh_target forwarding, got %#v", contract)
	}
	if contract.ResolutionStatus != "planned" {
		t.Fatalf("expected unhealthy remote rescue to remain planned before overlay sync, got %#v", contract)
	}
	if contract.FallbackClass != "network_down" || !contract.MeshHealthRequired {
		t.Fatalf("expected unhealthy mesh rescue to classify as network_down with mesh health required, got %#v", contract)
	}
	if !containsString(contract.InvalidationReasons, "overlay_route_sync_pending") {
		t.Fatalf("expected unhealthy mesh rescue to include overlay_route_sync_pending, got %#v", contract.InvalidationReasons)
	}
}

func TestValidateSidecarEnvContractRejectsMissingRequiredKey(t *testing.T) {
	serviceIndex := map[string]ServiceRuntimeContext{
		"api": {Name: "api"},
	}
	contract := SidecarEnvContract{
		Alias:         "api",
		TargetService: "api",
		Protocol:      "http",
		RequiredKeys: []string{
			"LAZYOPS_DEP_API_ENDPOINT",
			"LAZYOPS_DEP_API_PROTOCOL",
		},
		Values: map[string]string{
			"LAZYOPS_DEP_API_ENDPOINT": "http://localhost:8080",
		},
		SecretSafe: true,
	}

	err := validateSidecarEnvContract("web", contract, serviceIndex)
	if err == nil {
		t.Fatal("expected env contract validation to reject missing required key")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "sidecar_env_contract_missing_key" {
		t.Fatalf("expected sidecar_env_contract_missing_key, got %q", opErr.Code)
	}
}

func TestValidateSidecarEnvContractRejectsMissingTargetService(t *testing.T) {
	err := validateSidecarEnvContract("web", SidecarEnvContract{
		Alias:         "api",
		TargetService: "missing",
		Protocol:      "http",
		RequiredKeys:  []string{"LAZYOPS_DEP_API_ENDPOINT"},
		Values: map[string]string{
			"LAZYOPS_DEP_API_ENDPOINT": "http://localhost:8080",
		},
		SecretSafe: true,
	}, map[string]ServiceRuntimeContext{
		"api": {Name: "api"},
	})
	if err == nil {
		t.Fatal("expected env contract validation to reject missing target service")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "sidecar_env_contract_missing_target" {
		t.Fatalf("expected sidecar_env_contract_missing_target, got %q", opErr.Code)
	}
}

func TestValidateManagedCredentialContractRejectsPlaintextValue(t *testing.T) {
	err := validateManagedCredentialContract("web", SidecarManagedCredentialContract{
		Alias:         "api",
		TargetService: "api",
		Protocol:      "http",
		CredentialRef: "managed://prj_123/web/api",
		RequiredKeys: []string{
			"LAZYOPS_MANAGED_API_REF",
			"LAZYOPS_MANAGED_API_HANDLE",
		},
		Values: map[string]string{
			"LAZYOPS_MANAGED_API_REF":    "managed://prj_123/web/api",
			"LAZYOPS_MANAGED_API_HANDLE": "plain-secret-handle",
		},
		SecretSafe:             true,
		LocalhostRescueSkipped: true,
	}, map[string]ServiceRuntimeContext{
		"api": {Name: "api"},
	})
	if err == nil {
		t.Fatal("expected managed credential validation to reject plaintext handle")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "managed_credential_plaintext_forbidden" {
		t.Fatalf("expected managed_credential_plaintext_forbidden, got %q", opErr.Code)
	}
}

func TestValidateSidecarEnvContractRejectsSensitiveKeys(t *testing.T) {
	err := validateSidecarEnvContract("web", SidecarEnvContract{
		Alias:         "api",
		TargetService: "api",
		Protocol:      "http",
		RequiredKeys:  []string{"LAZYOPS_DEP_API_SECRET"},
		Values: map[string]string{
			"LAZYOPS_DEP_API_SECRET": "should-not-exist",
		},
		SecretSafe: true,
	}, map[string]ServiceRuntimeContext{
		"api": {Name: "api"},
	})
	if err == nil {
		t.Fatal("expected env contract validation to reject sensitive env key")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "sidecar_env_contract_secret_key" {
		t.Fatalf("expected sidecar_env_contract_secret_key, got %q", opErr.Code)
	}
}

func TestSelectSidecarModeKeepsEnvInjectionFirst(t *testing.T) {
	mode := selectSidecarMode(contracts.CompatibilityPolicy{
		EnvInjection:       true,
		ManagedCredentials: true,
		LocalhostRescue:    true,
	})
	if mode != "env_injection" {
		t.Fatalf("expected env injection to remain first precedence mode, got %q", mode)
	}
	if err := validateSelectedSidecarMode(mode, contracts.CompatibilityPolicy{
		EnvInjection:       true,
		ManagedCredentials: true,
		LocalhostRescue:    true,
	}); err != nil {
		t.Fatalf("expected precedence validation to pass, got %v", err)
	}
}

func TestSelectSidecarModeKeepsManagedCredentialsAheadOfLocalhostRescue(t *testing.T) {
	mode := selectSidecarMode(contracts.CompatibilityPolicy{
		EnvInjection:       false,
		ManagedCredentials: true,
		LocalhostRescue:    true,
	})
	if mode != "managed_credentials" {
		t.Fatalf("expected managed_credentials to remain ahead of localhost rescue, got %q", mode)
	}
	if err := validateSelectedSidecarMode(mode, contracts.CompatibilityPolicy{
		EnvInjection:       false,
		ManagedCredentials: true,
		LocalhostRescue:    true,
	}); err != nil {
		t.Fatalf("expected managed credential precedence validation to pass, got %v", err)
	}
}

func TestFilesystemDriverRenderSidecarsResolvesVerifiedRemoteMeshDependency(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	if driver.sidecar != nil {
		driver.sidecar.now = driver.now
	}

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := driver.mesh.EnsurePeer(context.Background(), contracts.EnsureMeshPeerPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
		PeerRef:     "mesh:mesh_remote",
		TargetID:    "mesh_remote",
		TargetKind:  contracts.TargetKindMesh,
	}); err != nil {
		t.Fatalf("ensure remote peer: %v", err)
	}
	if _, err := driver.mesh.SyncOverlayRoutes(context.Background(), contracts.SyncOverlayRoutesPayload{
		ProjectID:   payload.Project.ProjectID,
		BindingID:   payload.Binding.BindingID,
		RevisionID:  payload.Revision.RevisionID,
		RuntimeMode: payload.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
	}); err != nil {
		t.Fatalf("sync overlay routes: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	resolution := rendered.Plan.Services[0].Resolutions[0]
	if resolution.RouteScope != "mesh_private" {
		t.Fatalf("expected mesh_private route scope, got %#v", resolution)
	}
	if resolution.ResolutionStatus != "verified" {
		t.Fatalf("expected verified mesh resolution, got %#v", resolution)
	}
	if resolution.Provider != contracts.MeshProviderWireGuard {
		t.Fatalf("expected wireguard provider, got %#v", resolution)
	}
	if resolution.PlacementPeerRef != "mesh:mesh_remote" {
		t.Fatalf("expected remote placement peer mesh:mesh_remote, got %#v", resolution)
	}
	if !strings.Contains(rendered.Plan.Services[0].Env["LAZYOPS_DEP_API_ENDPOINT"], ".mesh.lazyops.internal") {
		t.Fatalf("expected env endpoint to use mesh host, got %#v", rendered.Plan.Services[0].Env)
	}
	if len(resolution.InvalidationReasons) != 0 {
		t.Fatalf("expected verified route to have no active invalidation reasons, got %#v", resolution.InvalidationReasons)
	}
}

func TestFilesystemDriverRenderSidecarsMarksPendingRemoteDependencyForInvalidation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 2, 13, 30, 0, 0, time.UTC)
	}
	if driver.mesh != nil {
		driver.mesh.now = driver.now
	}
	if driver.sidecar != nil {
		driver.sidecar.now = driver.now
	}

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	payload.Binding.TargetKind = contracts.TargetKindMesh
	payload.Binding.TargetID = "mesh_local"
	payload.Revision.PlacementAssignments = []contracts.PlacementAssignment{
		{
			ServiceName: "api",
			TargetID:    "mesh_remote",
			TargetKind:  contracts.TargetKindMesh,
		},
		{
			ServiceName: "web",
			TargetID:    "mesh_local",
			TargetKind:  contracts.TargetKindMesh,
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	resolution := rendered.Plan.Services[0].Resolutions[0]
	if resolution.ResolutionStatus != "planned" {
		t.Fatalf("expected planned resolution before overlay sync, got %#v", resolution)
	}
	if !containsString(resolution.InvalidationReasons, "overlay_route_sync_pending") {
		t.Fatalf("expected overlay_route_sync_pending invalidation reason, got %#v", resolution.InvalidationReasons)
	}
	if !containsString(resolution.InvalidationReasons, "mesh_peer_health_changed") {
		t.Fatalf("expected mesh_peer_health_changed invalidation reason, got %#v", resolution.InvalidationReasons)
	}
	if !strings.Contains(rendered.Plan.Services[0].Env["LAZYOPS_DEP_API_ENDPOINT"], ".mesh.lazyops.internal") {
		t.Fatalf("expected pending remote endpoint to already target mesh host, got %#v", rendered.Plan.Services[0].Env)
	}
}

func TestFilesystemDriverRenderSidecarsRemovesStaleServiceConfigOnVersionChange(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	firstCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build first runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), firstCtx); err != nil {
		t.Fatalf("prepare first release workspace: %v", err)
	}
	if _, err := driver.RenderSidecars(context.Background(), firstCtx); err != nil {
		t.Fatalf("render first sidecars: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "sidecars", "live", "services", "web", "config.json")); err != nil {
		t.Fatalf("expected live web sidecar config to exist after first render: %v", err)
	}

	secondPayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	secondPayload.Revision.RevisionID = "rev_124"
	secondPayload.Revision.DependencyBindings = nil
	secondCtx, err := ContextFromPreparePayload(secondPayload)
	if err != nil {
		t.Fatalf("build second runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), secondCtx); err != nil {
		t.Fatalf("prepare second release workspace: %v", err)
	}

	rendered, err := driver.RenderSidecars(context.Background(), secondCtx)
	if err != nil {
		t.Fatalf("render second sidecars: %v", err)
	}
	if rendered.Plan.Restart == nil || rendered.Plan.Restart.Status != "restarted" {
		t.Fatalf("expected sidecar restart on version change, got %#v", rendered.Plan.Restart)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "sidecars", "live", "services", "web")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale web sidecar config to be removed, got %v", err)
	}
}

func TestFilesystemDriverPromoteReleaseWritesTrafficShiftAndSummary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	}
	if driver.gateway != nil {
		driver.gateway.now = driver.now
	}
	if driver.sidecar != nil {
		driver.sidecar.now = driver.now
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)

	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	stableCtx, err := ContextFromPreparePayload(stablePayload)
	if err != nil {
		t.Fatalf("build stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), stableCtx); err != nil {
		t.Fatalf("prepare stable release workspace: %v", err)
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	runtimeCtx.Rollout.CurrentRevisionID = "rev_122"
	runtimeCtx.Rollout.StableRevisionID = "rev_122"

	preparePromotionReadyRuntime(t, driver, runtimeCtx)

	promoted, err := driver.PromoteRelease(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("promote release: %v", err)
	}
	if !promoted.ZeroDowntime {
		t.Fatal("expected zero-downtime promotion to be enabled by default")
	}
	if !promoted.RollbackReady {
		t.Fatal("expected rollback path to remain ready when previous stable exists")
	}
	if promoted.PreviousStableRevisionID != "rev_122" {
		t.Fatalf("expected previous stable rev_122, got %q", promoted.PreviousStableRevisionID)
	}
	for _, path := range []string{promoted.TrafficPath, promoted.DrainPlanPath, promoted.SummaryPath, promoted.EventsPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected promotion artifact %s to exist: %v", path, err)
		}
	}
	if promoted.Traffic.ActiveRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected active revision %q, got %q", payload.Revision.RevisionID, promoted.Traffic.ActiveRevisionID)
	}
	if promoted.DrainPlan.Status != "draining" {
		t.Fatalf("expected drain status draining, got %q", promoted.DrainPlan.Status)
	}
	if len(promoted.Events) != 2 {
		t.Fatalf("expected 2 deployment events, got %d", len(promoted.Events))
	}
	if promoted.Events[0].Type != "deployment.candidate_ready" || promoted.Events[1].Type != "deployment.promoted" {
		t.Fatalf("unexpected deployment event sequence: %#v", promoted.Events)
	}
	if len(promoted.Summary.LatencySignals) != 2 {
		t.Fatalf("expected latency signals for 2 services, got %d", len(promoted.Summary.LatencySignals))
	}

	historyRaw, err := os.ReadFile(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "rollout", "live", "history.json"))
	if err != nil {
		t.Fatalf("read rollout history: %v", err)
	}
	var history []PromotionSummary
	if err := json.Unmarshal(historyRaw, &history); err != nil {
		t.Fatalf("decode rollout history: %v", err)
	}
	if len(history) != 1 || history[0].RevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected rollout history to contain promoted revision, got %#v", history)
	}
}

func TestFilesystemDriverPromoteReleaseRejectsNonPromotableCandidate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := driver.RenderGatewayConfig(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("render gateway config: %v", err)
	}
	if _, err := driver.RenderSidecars(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	if _, err := driver.StartReleaseCandidate(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("start release candidate: %v", err)
	}

	_, err = driver.PromoteRelease(context.Background(), runtimeCtx)
	if err == nil {
		t.Fatal("expected promotion to fail before health gate marks candidate promotable")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "promotion_candidate_not_ready" {
		t.Fatalf("expected promotion_candidate_not_ready code, got %q", opErr.Code)
	}
	if opErr.Retryable {
		t.Fatal("expected non-promotable candidate error to be non-retryable")
	}
}

func TestFilesystemDriverRollbackReleaseReturnsTrafficToPreviousStableAndWritesIncident(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 10, 30, 0, 0, time.UTC)
	}
	if driver.gateway != nil {
		driver.gateway.now = driver.now
	}
	if driver.sidecar != nil {
		driver.sidecar.now = driver.now
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)

	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	stableCtx, err := ContextFromPreparePayload(stablePayload)
	if err != nil {
		t.Fatalf("build stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), stableCtx); err != nil {
		t.Fatalf("prepare stable release workspace: %v", err)
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	runtimeCtx.Rollout.CurrentRevisionID = "rev_122"
	runtimeCtx.Rollout.StableRevisionID = "rev_122"

	preparePromotionReadyRuntime(t, driver, runtimeCtx)
	if _, err := driver.PromoteRelease(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("promote release: %v", err)
	}

	runtimeCtx.Rollout.CurrentRevisionID = payload.Revision.RevisionID
	runtimeCtx.Rollout.StableRevisionID = payload.Revision.RevisionID
	runtimeCtx.Rollout.PreviousStableRevisionID = "rev_122"
	runtimeCtx.Rollout.DrainingRevisionID = "rev_122"

	rolledBack, err := driver.RollbackRelease(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("rollback release: %v", err)
	}
	if rolledBack.FailedRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected failed revision %q, got %q", payload.Revision.RevisionID, rolledBack.FailedRevisionID)
	}
	if rolledBack.RestoredRevisionID != "rev_122" {
		t.Fatalf("expected restored revision rev_122, got %q", rolledBack.RestoredRevisionID)
	}
	if rolledBack.Incident == nil {
		t.Fatal("expected rollback incident to be emitted")
	}
	if rolledBack.Incident.Severity != contracts.SeverityCritical {
		t.Fatalf("expected critical incident severity, got %q", rolledBack.Incident.Severity)
	}
	if rolledBack.Traffic.ActiveRevisionID != "rev_122" || rolledBack.Traffic.StableRevisionID != "rev_122" {
		t.Fatalf("expected traffic to return to rev_122, got %#v", rolledBack.Traffic)
	}
	if rolledBack.Traffic.PreviousRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected previous revision to track unhealthy revision %q, got %q", payload.Revision.RevisionID, rolledBack.Traffic.PreviousRevisionID)
	}
	if len(rolledBack.Events) != 2 {
		t.Fatalf("expected 2 rollback deployment events, got %d", len(rolledBack.Events))
	}
	if rolledBack.Events[0].Type != "deployment.unhealthy" || rolledBack.Events[1].Type != "deployment.rolled_back" {
		t.Fatalf("unexpected rollback event sequence: %#v", rolledBack.Events)
	}
	for _, path := range []string{rolledBack.TrafficPath, rolledBack.SummaryPath, rolledBack.IncidentPath, rolledBack.DrainPlanPath, rolledBack.EventsPath, rolledBack.RollbackPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected rollback artifact %s to exist: %v", path, err)
		}
	}

	candidate, err := loadCandidateRecord(workspaceLayout(root, runtimeCtx))
	if err != nil {
		t.Fatalf("load rolled back candidate record: %v", err)
	}
	if candidate.State != CandidateStateFailed {
		t.Fatalf("expected candidate audit state failed after rollback, got %q", candidate.State)
	}
	if candidate.LatestIncident == nil || candidate.LatestIncident.Kind != "deployment_promoted_revision_unhealthy" {
		t.Fatalf("expected candidate audit to retain rollback incident, got %#v", candidate.LatestIncident)
	}
}

func TestFilesystemDriverRollbackReleaseRejectsWhenNoPreviousStableExists(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 10, 45, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}

	preparePromotionReadyRuntime(t, driver, runtimeCtx)
	if _, err := driver.PromoteRelease(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("promote initial release: %v", err)
	}

	runtimeCtx.Rollout.CurrentRevisionID = payload.Revision.RevisionID
	runtimeCtx.Rollout.StableRevisionID = payload.Revision.RevisionID

	_, err = driver.RollbackRelease(context.Background(), runtimeCtx)
	if err == nil {
		t.Fatal("expected rollback to fail without a previous stable revision")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected operation error, got %T", err)
	}
	if opErr.Code != "rollback_previous_stable_missing" {
		t.Fatalf("expected rollback_previous_stable_missing code, got %q", opErr.Code)
	}
	if opErr.Retryable {
		t.Fatal("expected missing previous stable rollback failure to be non-retryable")
	}
}

func TestFilesystemDriverRunHealthGateMarksCandidatePromotable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	payload.Revision.Services[0].HealthCheck.SuccessThreshold = 2
	payload.Revision.Services[0].HealthCheck.Timeout = "400ms"

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := driver.StartReleaseCandidate(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("start release candidate: %v", err)
	}

	report, err := driver.RunHealthGate(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("run health gate: %v", err)
	}
	if !report.Promotable {
		t.Fatal("expected candidate to become promotable")
	}
	if report.CandidateState != CandidateStatePromotable {
		t.Fatalf("expected promotable candidate state, got %q", report.CandidateState)
	}
	if report.PolicyAction != HealthGatePolicyPromoteCandidate {
		t.Fatalf("expected promote policy action, got %q", report.PolicyAction)
	}
	apiHealth := findHealthResult(t, report.Services, "api")
	if apiHealth.Attempts < 2 {
		t.Fatalf("expected api health gate to require at least 2 attempts, got %d", apiHealth.Attempts)
	}
	if apiHealth.Successes != 2 {
		t.Fatalf("expected api health successes to reach threshold 2, got %d", apiHealth.Successes)
	}
	if _, err := os.Stat(report.ReportPath); err != nil {
		t.Fatalf("expected health gate report to exist: %v", err)
	}
	if _, err := os.Stat(report.RolloutSummaryPath); err != nil {
		t.Fatalf("expected rollout summary to exist: %v", err)
	}

	candidate, err := loadCandidateRecord(workspaceLayout(root, runtimeCtx))
	if err != nil {
		t.Fatalf("load candidate record: %v", err)
	}
	if candidate.State != CandidateStatePromotable {
		t.Fatalf("expected candidate manifest state promotable, got %q", candidate.State)
	}
	if len(candidate.History) < 4 {
		t.Fatalf("expected candidate history to include health transitions, got %d transitions", len(candidate.History))
	}
}

func TestFilesystemDriverRunHealthGateFailsWithRollbackPolicyAndDedupesIncident(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 8, 30, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	payload.Revision.Services[0].HealthCheck.FailureThreshold = 1

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	runtimeCtx.Rollout.StableRevisionID = "rev_stable"
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := driver.StartReleaseCandidate(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("start release candidate: %v", err)
	}

	report, err := driver.RunHealthGate(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("run health gate: %v", err)
	}
	if report.Promotable {
		t.Fatal("expected candidate to fail health gate")
	}
	if report.CandidateState != CandidateStateFailed {
		t.Fatalf("expected failed candidate state, got %q", report.CandidateState)
	}
	if report.PolicyAction != HealthGatePolicyRollbackRelease {
		t.Fatalf("expected rollback policy action, got %q", report.PolicyAction)
	}
	if report.Incident == nil {
		t.Fatal("expected first failing health gate to produce incident payload")
	}

	secondReport, err := driver.RunHealthGate(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("rerun health gate: %v", err)
	}
	if !secondReport.IncidentSuppressed {
		t.Fatal("expected duplicate failing incident to be suppressed")
	}

	candidate, err := loadCandidateRecord(workspaceLayout(root, runtimeCtx))
	if err != nil {
		t.Fatalf("load candidate record: %v", err)
	}
	if candidate.State != CandidateStateFailed {
		t.Fatalf("expected candidate manifest state failed, got %q", candidate.State)
	}
	if candidate.LatestIncident == nil {
		t.Fatal("expected candidate manifest to retain latest incident payload")
	}
}

func TestFilesystemDriverGarbageCollectRuntimeRemovesOnlyUnprotectedState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 4, 1, 11, 0, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	stalePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stalePayload.Revision.RevisionID = "rev_120"
	configureServiceHealthChecks(t, &stalePayload, server, tcpListener)
	staleCtx, err := ContextFromPreparePayload(stalePayload)
	if err != nil {
		t.Fatalf("build stale runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), staleCtx); err != nil {
		t.Fatalf("prepare stale release workspace: %v", err)
	}
	if _, err := driver.RenderGatewayConfig(context.Background(), staleCtx); err != nil {
		t.Fatalf("render stale gateway config: %v", err)
	}
	if _, err := driver.RenderSidecars(context.Background(), staleCtx); err != nil {
		t.Fatalf("render stale sidecars: %v", err)
	}

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	stableCtx, err := ContextFromPreparePayload(stablePayload)
	if err != nil {
		t.Fatalf("build stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), stableCtx); err != nil {
		t.Fatalf("prepare stable release workspace: %v", err)
	}
	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	runtimeCtx.Rollout.CurrentRevisionID = "rev_122"
	runtimeCtx.Rollout.StableRevisionID = "rev_122"
	preparePromotionReadyRuntime(t, driver, runtimeCtx)
	if _, err := driver.PromoteRelease(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("promote release: %v", err)
	}
	runtimeCtx.Rollout.CurrentRevisionID = payload.Revision.RevisionID
	runtimeCtx.Rollout.StableRevisionID = payload.Revision.RevisionID
	runtimeCtx.Rollout.PreviousStableRevisionID = "rev_122"
	if _, err := driver.RollbackRelease(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("rollback release: %v", err)
	}

	runtimeCtx.Rollout.CurrentRevisionID = "rev_122"
	runtimeCtx.Rollout.StableRevisionID = "rev_122"
	runtimeCtx.Rollout.PreviousStableRevisionID = ""
	runtimeCtx.Rollout.DrainingRevisionID = payload.Revision.RevisionID
	runtimeCtx.Revision.RevisionID = "rev_122"

	collected, err := driver.GarbageCollectRuntime(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("garbage collect runtime: %v", err)
	}
	if len(collected.ProtectedRevisionIDs) == 0 {
		t.Fatal("expected protected revision list to be recorded")
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "revisions", "rev_120")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale revision rev_120 to be removed, got %v", err)
	}
	for _, revisionID := range []string{"rev_122", "rev_123"} {
		if _, err := os.Stat(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "revisions", revisionID)); err != nil {
			t.Fatalf("expected protected revision %s to remain, got %v", revisionID, err)
		}
	}
	if len(collected.RemovedGatewayVersions) == 0 {
		t.Fatal("expected stale gateway version directories to be removed")
	}
	if len(collected.RemovedSidecarVersions) == 0 {
		t.Fatal("expected stale sidecar version directories to be removed")
	}
	if _, err := os.Stat(collected.ReportPath); err != nil {
		t.Fatalf("expected garbage collection report to exist: %v", err)
	}
}

func TestPrepareReleaseWorkspaceHandlerReturnsValidationErrorForBadPayload(t *testing.T) {
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, NewFilesystemDriver(nil, filepath.Join(t.TempDir(), "runtime-root")))

	result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_bad",
		CorrelationID: "corr_bad",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       json.RawMessage(`{"project":{},"binding":{},"revision":{}}`),
	})
	if result.Error == nil {
		t.Fatal("expected validation failure for incomplete payload")
	}
	if result.Error.Retryable {
		t.Fatal("expected invalid runtime context to be non-retryable")
	}
}

func TestPrepareReleaseWorkspaceHandlerCanBeRegistered(t *testing.T) {
	registry := dispatcher.NewDefaultRegistry()
	service := NewService(nil, nil, NewFilesystemDriver(nil, filepath.Join(t.TempDir(), "runtime-root")))
	service.Register(registry)

	if _, ok := registry.Resolve(contracts.CommandPrepareReleaseWorkspace); !ok {
		t.Fatal("expected runtime service to register prepare_release_workspace handler")
	}
	if _, ok := registry.Resolve(contracts.CommandRenderGatewayConfig); !ok {
		t.Fatal("expected runtime service to register render_gateway_config handler")
	}
	if _, ok := registry.Resolve(contracts.CommandRenderSidecars); !ok {
		t.Fatal("expected runtime service to register render_sidecars handler")
	}
	if _, ok := registry.Resolve(contracts.CommandStartReleaseCandidate); !ok {
		t.Fatal("expected runtime service to register start_release_candidate handler")
	}
	if _, ok := registry.Resolve(contracts.CommandRunHealthGate); !ok {
		t.Fatal("expected runtime service to register run_health_gate handler")
	}
	if _, ok := registry.Resolve(contracts.CommandPromoteRelease); !ok {
		t.Fatal("expected runtime service to register promote_release handler")
	}
	if _, ok := registry.Resolve(contracts.CommandRollbackRelease); !ok {
		t.Fatal("expected runtime service to register rollback_release handler")
	}
	if _, ok := registry.Resolve(contracts.CommandGarbageCollectRuntime); !ok {
		t.Fatal("expected runtime service to register garbage_collect_runtime handler")
	}
}

func TestStartReleaseCandidateHandlerUpdatesCandidateState(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), filepath.Join(t.TempDir(), "runtime-root"))
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_prepare",
		CorrelationID: "corr_prepare",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	}); result.Error != nil {
		t.Fatalf("prepare workspace failed: %#v", result.Error)
	}

	result := service.handleStartReleaseCandidate(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandStartReleaseCandidate,
		RequestID:     "req_start",
		CorrelationID: "corr_start",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected start candidate to succeed, got %#v", result.Error)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.CandidateRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected candidate revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.CandidateRevisionID)
	}
	if local.RevisionCache.CandidateState != "starting" {
		t.Fatalf("expected candidate state starting, got %q", local.RevisionCache.CandidateState)
	}
	if local.RevisionCache.CandidateWorkspaceRoot == "" {
		t.Fatal("expected candidate workspace root to be persisted")
	}
}

func TestPrepareReleaseWorkspaceCleansUpIncompleteWorkspaceOnFetchFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(nil, root)
	driver.fetcher = failingFetcher{err: errors.New("fetch failed")}

	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}

	_, err = driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx)
	if err == nil {
		t.Fatal("expected prepare release workspace to fail when fetcher fails")
	}

	layout := workspaceLayout(root, runtimeCtx)
	if _, statErr := os.Stat(layout.Root); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected incomplete workspace root to be removed, got %v", statErr)
	}
}

func TestRenderGatewayConfigHandlerAppliesGatewayConfig(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_prepare_gateway",
		CorrelationID: "corr_prepare_gateway",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	}); result.Error != nil {
		t.Fatalf("prepare workspace failed: %#v", result.Error)
	}

	result := service.handleRenderGatewayConfig(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRenderGatewayConfig,
		RequestID:     "req_gateway",
		CorrelationID: "corr_gateway",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected render gateway config to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	active, err := loadGatewayActivation(filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "gateway", "live", "active.json"))
	if err != nil {
		t.Fatalf("load active gateway activation: %v", err)
	}
	if active.Version == "" {
		t.Fatal("expected live gateway activation to record a version")
	}
}

func TestRenderGatewayConfigHandlerReturnsNonRetryableValidationError(t *testing.T) {
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, NewFilesystemDriver(nil, filepath.Join(t.TempDir(), "runtime-root")))

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	payload.Revision.Services[1].HealthCheck.Port = 0
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_prepare_gateway_invalid",
		CorrelationID: "corr_prepare_gateway_invalid",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	}); result.Error != nil {
		t.Fatalf("prepare workspace failed: %#v", result.Error)
	}

	result := service.handleRenderGatewayConfig(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRenderGatewayConfig,
		RequestID:     "req_gateway_invalid",
		CorrelationID: "corr_gateway_invalid",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected render gateway config to fail validation")
	}
	if result.Error.Retryable {
		t.Fatal("expected invalid gateway config to be non-retryable")
	}
	if result.Error.Code != "gateway_invalid_route_port" {
		t.Fatalf("expected gateway_invalid_route_port code, got %q", result.Error.Code)
	}
}

func TestRenderSidecarsHandlerAppliesSidecarConfig(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_prepare_sidecar",
		CorrelationID: "corr_prepare_sidecar",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	}); result.Error != nil {
		t.Fatalf("prepare workspace failed: %#v", result.Error)
	}

	result := service.handleRenderSidecars(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRenderSidecars,
		RequestID:     "req_sidecar",
		CorrelationID: "corr_sidecar",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected render sidecars to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	if _, err := os.Stat(filepath.Join(root, "cache", "sidecars", "prj_123", "bind_123", "metadata.json")); err != nil {
		t.Fatalf("expected sidecar metadata cache to exist: %v", err)
	}
}

func TestRenderSidecarsHandlerReturnsNonRetryableValidationError(t *testing.T) {
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, NewFilesystemDriver(nil, filepath.Join(t.TempDir(), "runtime-root")))

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	payload.Revision.DependencyBindings[0].TargetService = "missing-service"
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_prepare_sidecar_invalid",
		CorrelationID: "corr_prepare_sidecar_invalid",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	}); result.Error != nil {
		t.Fatalf("prepare workspace failed: %#v", result.Error)
	}

	result := service.handleRenderSidecars(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRenderSidecars,
		RequestID:     "req_sidecar_invalid",
		CorrelationID: "corr_sidecar_invalid",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected render sidecars to fail validation")
	}
	if result.Error.Retryable {
		t.Fatal("expected invalid sidecar config to be non-retryable")
	}
	if result.Error.Code != "sidecar_missing_target_service" {
		t.Fatalf("expected sidecar_missing_target_service code, got %q", result.Error.Code)
	}
}

func TestPromoteReleaseHandlerUpdatesRevisionCacheAndRolloutState(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.CurrentRevisionID = "rev_122"
		local.RevisionCache.StableRevisionID = "rev_122"
		return nil
	}); err != nil {
		t.Fatalf("seed stable/current revision: %v", err)
	}

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	stableCtx, err := ContextFromPreparePayload(stablePayload)
	if err != nil {
		t.Fatalf("build stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), stableCtx); err != nil {
		t.Fatalf("prepare stable release workspace: %v", err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runPromotionReadySetup(t, service, raw)

	result := service.handlePromoteRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPromoteRelease,
		RequestID:     "req_promote",
		CorrelationID: "corr_promote",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected promote release to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.CurrentRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected current revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.CurrentRevisionID)
	}
	if local.RevisionCache.StableRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected stable revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.StableRevisionID)
	}
	if local.RevisionCache.PreviousStableRevisionID != "rev_122" {
		t.Fatalf("expected previous stable revision rev_122, got %q", local.RevisionCache.PreviousStableRevisionID)
	}
	if local.RevisionCache.DrainingRevisionID != "rev_122" {
		t.Fatalf("expected draining revision rev_122, got %q", local.RevisionCache.DrainingRevisionID)
	}
	if local.RevisionCache.CandidateRevisionID != "" || local.RevisionCache.CandidateState != "" {
		t.Fatalf("expected candidate state to be cleared after promotion, got %#v", local.RevisionCache)
	}
	if local.RevisionCache.LastPromotionSummary == "" {
		t.Fatal("expected last promotion summary to be stored")
	}
}

func TestPromoteReleaseHandlerReturnsNonRetryableErrorWhenHealthGateNotPassed(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.CurrentRevisionID = "rev_122"
		local.RevisionCache.StableRevisionID = "rev_122"
		return nil
	}); err != nil {
		t.Fatalf("seed stable/current revision: %v", err)
	}

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runRuntimeCommands(t, service, raw,
		contracts.CommandPrepareReleaseWorkspace,
		contracts.CommandRenderGatewayConfig,
		contracts.CommandRenderSidecars,
		contracts.CommandStartReleaseCandidate,
	)

	result := service.handlePromoteRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPromoteRelease,
		RequestID:     "req_promote_invalid",
		CorrelationID: "corr_promote_invalid",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected promote release to fail when candidate is not promotable")
	}
	if result.Error.Retryable {
		t.Fatal("expected non-promotable promotion failure to be non-retryable")
	}
	if result.Error.Code != "promotion_candidate_not_ready" {
		t.Fatalf("expected promotion_candidate_not_ready code, got %q", result.Error.Code)
	}
}

func TestRollbackReleaseHandlerRestoresStableRevisionAndStoresRollbackAudit(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)
	service.now = func() time.Time {
		return time.Date(2026, 4, 1, 10, 35, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.CurrentRevisionID = "rev_122"
		local.RevisionCache.StableRevisionID = "rev_122"
		return nil
	}); err != nil {
		t.Fatalf("seed stable/current revision: %v", err)
	}

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	stableCtx, err := ContextFromPreparePayload(stablePayload)
	if err != nil {
		t.Fatalf("build stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), stableCtx); err != nil {
		t.Fatalf("prepare stable release workspace: %v", err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runPromotionReadySetup(t, service, raw)
	if result := service.handlePromoteRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPromoteRelease,
		RequestID:     "req_promote_for_rollback",
		CorrelationID: "corr_promote_for_rollback",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	}); result.Error != nil {
		t.Fatalf("promote release failed: %#v", result.Error)
	}

	result := service.handleRollbackRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRollbackRelease,
		RequestID:     "req_rollback",
		CorrelationID: "corr_rollback",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected rollback release to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.CurrentRevisionID != "rev_122" {
		t.Fatalf("expected current revision rev_122, got %q", local.RevisionCache.CurrentRevisionID)
	}
	if local.RevisionCache.StableRevisionID != "rev_122" {
		t.Fatalf("expected stable revision rev_122, got %q", local.RevisionCache.StableRevisionID)
	}
	if local.RevisionCache.DrainingRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected draining revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.DrainingRevisionID)
	}
	if local.RevisionCache.LastPolicyAction != string(HealthGatePolicyRollbackRelease) {
		t.Fatalf("expected rollback policy action, got %q", local.RevisionCache.LastPolicyAction)
	}
	if local.RevisionCache.LastRollbackFromRevision != payload.Revision.RevisionID {
		t.Fatalf("expected rollback from revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.LastRollbackFromRevision)
	}
	if local.RevisionCache.LastRollbackToRevision != "rev_122" {
		t.Fatalf("expected rollback to revision rev_122, got %q", local.RevisionCache.LastRollbackToRevision)
	}
	if local.RevisionCache.LastRollbackSummary == "" {
		t.Fatal("expected rollback summary to be stored")
	}
}

func TestGarbageCollectRuntimeHandlerStoresLastRunSummary(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)
	service.now = func() time.Time {
		return time.Date(2026, 4, 1, 11, 5, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	stalePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stalePayload.Revision.RevisionID = "rev_120"
	configureServiceHealthChecks(t, &stalePayload, server, tcpListener)
	staleRaw, err := json.Marshal(stalePayload)
	if err != nil {
		t.Fatalf("marshal stale payload: %v", err)
	}
	runRuntimeCommands(t, service, staleRaw,
		contracts.CommandPrepareReleaseWorkspace,
		contracts.CommandRenderGatewayConfig,
		contracts.CommandRenderSidecars,
	)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.CurrentRevisionID = "rev_122"
		local.RevisionCache.StableRevisionID = "rev_122"
		local.RevisionCache.DrainingRevisionID = payload.Revision.RevisionID
		return nil
	}); err != nil {
		t.Fatalf("seed runtime gc protected state: %v", err)
	}

	runRuntimeCommands(t, service, raw,
		contracts.CommandPrepareReleaseWorkspace,
		contracts.CommandRenderGatewayConfig,
		contracts.CommandRenderSidecars,
	)

	gcPayload := payload
	gcPayload.Revision.RevisionID = "rev_122"
	gcRaw, err := json.Marshal(gcPayload)
	if err != nil {
		t.Fatalf("marshal gc payload: %v", err)
	}

	result := service.handleGarbageCollectRuntime(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandGarbageCollectRuntime,
		RequestID:     "req_gc",
		CorrelationID: "corr_gc",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       gcRaw,
	})
	if result.Error != nil {
		t.Fatalf("expected garbage collect runtime to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.LastRuntimeGCSummary == "" {
		t.Fatal("expected runtime gc summary to be stored")
	}
	if local.RevisionCache.LastRuntimeGCAt.IsZero() {
		t.Fatal("expected runtime gc timestamp to be stored")
	}
}

func TestRunHealthGateHandlerUpdatesLocalStateOnSuccess(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), filepath.Join(t.TempDir(), "runtime-root"))
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runRuntimeSetup(t, service, raw)

	result := service.handleRunHealthGate(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRunHealthGate,
		RequestID:     "req_health",
		CorrelationID: "corr_health",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected health gate handler to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.CandidateState != string(CandidateStatePromotable) {
		t.Fatalf("expected promotable candidate state, got %q", local.RevisionCache.CandidateState)
	}
	if local.RevisionCache.LastPolicyAction != string(HealthGatePolicyPromoteCandidate) {
		t.Fatalf("expected promote policy action, got %q", local.RevisionCache.LastPolicyAction)
	}
	if local.RevisionCache.LastHealthGateSummary == "" {
		t.Fatal("expected health gate summary to be stored")
	}
}

func TestRunHealthGateHandlerReturnsRollbackDirectiveOnFailure(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), filepath.Join(t.TempDir(), "runtime-root"))
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.StableRevisionID = "rev_stable"
		return nil
	}); err != nil {
		t.Fatalf("seed stable revision: %v", err)
	}

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runRuntimeSetup(t, service, raw)

	result := service.handleRunHealthGate(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRunHealthGate,
		RequestID:     "req_health_fail",
		CorrelationID: "corr_health_fail",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected health gate handler to return failure directive")
	}
	if result.Error.Retryable {
		t.Fatal("expected health gate failure to be non-retryable")
	}
	if result.Error.Code != "health_gate_failed" {
		t.Fatalf("expected health_gate_failed code, got %q", result.Error.Code)
	}
	if result.Error.Details["policy_action"] != HealthGatePolicyRollbackRelease {
		t.Fatalf("expected rollback_release policy action, got %#v", result.Error.Details["policy_action"])
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.CandidateState != string(CandidateStateFailed) {
		t.Fatalf("expected failed candidate state, got %q", local.RevisionCache.CandidateState)
	}
	if local.RevisionCache.LastPolicyAction != string(HealthGatePolicyRollbackRelease) {
		t.Fatalf("expected rollback_release policy action in state, got %q", local.RevisionCache.LastPolicyAction)
	}
}

func samplePreparePayload(mode contracts.RuntimeMode) contracts.PrepareReleaseWorkspacePayload {
	return contracts.PrepareReleaseWorkspacePayload{
		Project: contracts.ProjectMetadataPayload{
			ProjectID: "prj_123",
			Name:      "Lazy App",
			Slug:      "lazy-app",
		},
		Binding: contracts.DeploymentBindingPayload{
			BindingID:   "bind_123",
			ProjectID:   "prj_123",
			Name:        "prod",
			TargetRef:   "prod-main",
			RuntimeMode: mode,
			TargetKind:  contracts.TargetKindInstance,
			TargetID:    "inst_123",
			PlacementPolicy: contracts.PlacementPolicy{
				Strategy: "spread",
			},
			DomainPolicy: contracts.DomainPolicy{
				Enabled:  true,
				Provider: "sslip.io",
			},
			CompatibilityPolicy: contracts.CompatibilityPolicy{
				EnvInjection:       true,
				ManagedCredentials: true,
				LocalhostRescue:    true,
			},
			ScaleToZeroPolicy: contracts.ScaleToZeroPolicy{
				Enabled: false,
			},
		},
		Revision: contracts.DesiredRevisionPayload{
			RevisionID:          "rev_123",
			ProjectID:           "prj_123",
			BlueprintID:         "bp_123",
			DeploymentBindingID: "bind_123",
			CommitSHA:           "abc123",
			ArtifactRef:         "artifact://lazy-app/rev_123.tar.gz",
			ImageRef:            "ghcr.io/lazyops/lazy-app:rev_123",
			TriggerKind:         "git_push",
			RuntimeMode:         mode,
			Services: []contracts.ServicePayload{
				{
					Name:   "api",
					Path:   "services/api",
					Public: false,
					HealthCheck: contracts.HealthCheckPayload{
						Protocol: "http",
						Port:     8080,
						Path:     "/health",
					},
				},
				{
					Name:   "web",
					Path:   "services/web",
					Public: true,
					HealthCheck: contracts.HealthCheckPayload{
						Protocol: "http",
						Port:     3000,
						Path:     "/ready",
					},
				},
			},
			DependencyBindings: []contracts.DependencyBindingPayload{
				{
					Service:       "web",
					Alias:         "api",
					TargetService: "api",
					Protocol:      "http",
					LocalEndpoint: "http://localhost:8080",
				},
			},
			CompatibilityPolicy: contracts.CompatibilityPolicy{
				EnvInjection:       true,
				ManagedCredentials: true,
				LocalhostRescue:    true,
			},
			MagicDomainPolicy: contracts.MagicDomainPolicy{
				Enabled:  true,
				Provider: "sslip.io",
			},
			ScaleToZeroPolicy: contracts.ScaleToZeroPolicy{
				Enabled: false,
			},
			PlacementAssignments: []contracts.PlacementAssignment{
				{
					ServiceName: "api",
					TargetID:    "inst_123",
					TargetKind:  contracts.TargetKindInstance,
				},
			},
		},
	}
}

type failingFetcher struct {
	err error
}

func (f failingFetcher) FetchRevisionAssets(context.Context, RuntimeContext, WorkspaceLayout) (ArtifactMaterialization, error) {
	return ArtifactMaterialization{}, f.err
}

func configureServiceHealthChecks(t *testing.T, payload *contracts.PrepareReleaseWorkspacePayload, httpServer *httptest.Server, tcpListener net.Listener) {
	t.Helper()

	httpAddr, ok := httpServer.Listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("expected http test server to use TCP listener")
	}
	tcpAddr, ok := tcpListener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("expected tcp listener to use TCP addr")
	}

	payload.Revision.Services[0].HealthCheck.Protocol = "http"
	payload.Revision.Services[0].HealthCheck.Port = httpAddr.Port
	payload.Revision.Services[0].HealthCheck.Path = "/health"
	payload.Revision.Services[0].HealthCheck.Timeout = "400ms"
	payload.Revision.Services[0].HealthCheck.SuccessThreshold = 1
	payload.Revision.Services[0].HealthCheck.FailureThreshold = 1

	payload.Revision.Services[1].HealthCheck.Protocol = "tcp"
	payload.Revision.Services[1].HealthCheck.Port = tcpAddr.Port
	payload.Revision.Services[1].HealthCheck.Path = ""
	payload.Revision.Services[1].HealthCheck.Timeout = "400ms"
	payload.Revision.Services[1].HealthCheck.SuccessThreshold = 1
	payload.Revision.Services[1].HealthCheck.FailureThreshold = 1
}

func startTCPHealthListener(t *testing.T) (net.Listener, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on tcp health port: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	return listener, func() {
		_ = listener.Close()
		<-done
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func runRuntimeSetup(t *testing.T, service *Service, raw []byte) {
	t.Helper()

	runRuntimeCommands(t, service, raw, contracts.CommandPrepareReleaseWorkspace, contracts.CommandStartReleaseCandidate)
}

func runPromotionReadySetup(t *testing.T, service *Service, raw []byte) {
	t.Helper()

	runRuntimeCommands(t, service, raw,
		contracts.CommandPrepareReleaseWorkspace,
		contracts.CommandRenderGatewayConfig,
		contracts.CommandRenderSidecars,
		contracts.CommandStartReleaseCandidate,
		contracts.CommandRunHealthGate,
	)
}

func runRuntimeCommands(t *testing.T, service *Service, raw []byte, commands ...contracts.CommandType) {
	t.Helper()

	for _, command := range commands {
		envelope := contracts.CommandEnvelope{
			Type:          command,
			RequestID:     "req_" + string(command),
			CorrelationID: "corr_" + string(command),
			AgentID:       "agt_local",
			Source:        contracts.EnvelopeSourceBackend,
			OccurredAt:    time.Now().UTC(),
			Payload:       raw,
		}
		var result dispatcher.Result
		switch command {
		case contracts.CommandPrepareReleaseWorkspace:
			result = service.handlePrepareReleaseWorkspace(context.Background(), envelope)
		case contracts.CommandRenderGatewayConfig:
			result = service.handleRenderGatewayConfig(context.Background(), envelope)
		case contracts.CommandRenderSidecars:
			result = service.handleRenderSidecars(context.Background(), envelope)
		case contracts.CommandStartReleaseCandidate:
			result = service.handleStartReleaseCandidate(context.Background(), envelope)
		case contracts.CommandRunHealthGate:
			result = service.handleRunHealthGate(context.Background(), envelope)
		case contracts.CommandPromoteRelease:
			result = service.handlePromoteRelease(context.Background(), envelope)
		case contracts.CommandRollbackRelease:
			result = service.handleRollbackRelease(context.Background(), envelope)
		case contracts.CommandGarbageCollectRuntime:
			result = service.handleGarbageCollectRuntime(context.Background(), envelope)
		default:
			t.Fatalf("unsupported runtime command in test helper: %s", command)
		}
		if result.Error != nil {
			t.Fatalf("runtime command %s failed: %#v", command, result.Error)
		}
	}
}

func preparePromotionReadyRuntime(t *testing.T, driver *FilesystemDriver, runtimeCtx RuntimeContext) {
	t.Helper()

	if _, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}
	if _, err := driver.RenderGatewayConfig(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("render gateway config: %v", err)
	}
	if _, err := driver.RenderSidecars(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("render sidecars: %v", err)
	}
	if _, err := driver.StartReleaseCandidate(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("start release candidate: %v", err)
	}
	if _, err := driver.RunHealthGate(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("run health gate: %v", err)
	}
}

func findHealthResult(t *testing.T, results []ServiceHealthResult, serviceName string) ServiceHealthResult {
	t.Helper()
	for _, result := range results {
		if result.ServiceName == serviceName {
			return result
		}
	}
	t.Fatalf("health result for service %q not found", serviceName)
	return ServiceHealthResult{}
}
