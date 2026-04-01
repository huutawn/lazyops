package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
)

type MeshPeerState string

const (
	MeshPeerStatePlanned  MeshPeerState = "planned"
	MeshPeerStateJoining  MeshPeerState = "joining"
	MeshPeerStateActive   MeshPeerState = "active"
	MeshPeerStateDegraded MeshPeerState = "degraded"
	MeshPeerStateLeaving  MeshPeerState = "leaving"
	MeshPeerStateRemoved  MeshPeerState = "removed"
)

type MeshPeer struct {
	PeerRef    string               `json:"peer_ref"`
	TargetID   string               `json:"target_id"`
	TargetKind contracts.TargetKind `json:"target_kind"`
	Local      bool                 `json:"local"`
	State      MeshPeerState        `json:"state"`
	Services   []string             `json:"services,omitempty"`
	UpdatedAt  time.Time            `json:"updated_at"`
}

type MeshMembershipState struct {
	LocalPeerRef string        `json:"local_peer_ref"`
	LocalState   MeshPeerState `json:"local_state"`
	Peers        []MeshPeer    `json:"peers,omitempty"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type MeshRouteRecord struct {
	RouteKey       string `json:"route_key"`
	ServiceName    string `json:"service_name"`
	Alias          string `json:"alias"`
	TargetService  string `json:"target_service"`
	Protocol       string `json:"protocol"`
	SourceTargetID string `json:"source_target_id"`
	TargetTargetID string `json:"target_target_id"`
	SourcePeerRef  string `json:"source_peer_ref"`
	TargetPeerRef  string `json:"target_peer_ref"`
	PathKind       string `json:"path_kind"`
	Status         string `json:"status"`
	PrivateOnly    bool   `json:"private_only"`
	LocalEndpoint  string `json:"local_endpoint,omitempty"`
}

type MeshHealthSummary struct {
	Status        string    `json:"status"`
	TotalPeers    int       `json:"total_peers"`
	ActivePeers   int       `json:"active_peers"`
	PlannedPeers  int       `json:"planned_peers"`
	DegradedPeers int       `json:"degraded_peers"`
	TotalRoutes   int       `json:"total_routes"`
	LocalRoutes   int       `json:"local_routes"`
	PrivateRoutes int       `json:"private_routes"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ServiceMetadataCache struct {
	Version   string                           `json:"version"`
	UpdatedAt time.Time                        `json:"updated_at"`
	Services  map[string]ServiceMetadataRecord `json:"services"`
}

type ServiceMetadataRecord struct {
	ServiceName         string                      `json:"service_name"`
	Public              bool                        `json:"public"`
	PlacementTargetID   string                      `json:"placement_target_id"`
	PlacementTargetKind contracts.TargetKind        `json:"placement_target_kind"`
	PlacementPeerRef    string                      `json:"placement_peer_ref"`
	RouteScope          string                      `json:"route_scope"`
	Dependencies        []ServiceDependencyMetadata `json:"dependencies,omitempty"`
}

type ServiceDependencyMetadata struct {
	Alias            string               `json:"alias"`
	TargetService    string               `json:"target_service"`
	Protocol         string               `json:"protocol"`
	LocalEndpoint    string               `json:"local_endpoint,omitempty"`
	SourceTargetID   string               `json:"source_target_id"`
	SourceTargetKind contracts.TargetKind `json:"source_target_kind"`
	SourcePeerRef    string               `json:"source_peer_ref"`
	TargetTargetID   string               `json:"target_target_id"`
	TargetTargetKind contracts.TargetKind `json:"target_target_kind"`
	TargetPeerRef    string               `json:"target_peer_ref"`
	RouteScope       string               `json:"route_scope"`
	PrivateOnly      bool                 `json:"private_only"`
}

type MeshFoundationSnapshot struct {
	Enabled     bool                   `json:"enabled"`
	ProjectID   string                 `json:"project_id"`
	BindingID   string                 `json:"binding_id"`
	RuntimeMode contracts.RuntimeMode  `json:"runtime_mode"`
	Provider    contracts.MeshProvider `json:"provider,omitempty"`
	Membership  MeshMembershipState    `json:"membership"`
	RouteCache  []MeshRouteRecord      `json:"route_cache,omitempty"`
	Health      MeshHealthSummary      `json:"health"`
	GeneratedAt time.Time              `json:"generated_at"`
}

type MeshFoundationResult struct {
	Snapshot                  MeshFoundationSnapshot `json:"snapshot"`
	WorkspaceStatePath        string                 `json:"workspace_state_path,omitempty"`
	WorkspaceServiceCachePath string                 `json:"workspace_service_cache_path,omitempty"`
	LiveStatePath             string                 `json:"live_state_path,omitempty"`
	LiveServiceCachePath      string                 `json:"live_service_cache_path,omitempty"`
}

type MeshManager struct {
	logger      *slog.Logger
	runtimeRoot string
	now         func() time.Time
}

func NewMeshManager(logger *slog.Logger, runtimeRoot string) *MeshManager {
	return &MeshManager{
		logger:      logger,
		runtimeRoot: runtimeRoot,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (m *MeshManager) BuildFoundation(_ context.Context, runtimeCtx RuntimeContext, layout WorkspaceLayout) (MeshFoundationResult, error) {
	now := m.now()
	if runtimeCtx.Binding.RuntimeMode != contracts.RuntimeModeDistributedMesh {
		return MeshFoundationResult{
			Snapshot: MeshFoundationSnapshot{
				Enabled:     false,
				ProjectID:   runtimeCtx.Project.ProjectID,
				BindingID:   runtimeCtx.Binding.BindingID,
				RuntimeMode: runtimeCtx.Binding.RuntimeMode,
				Health: MeshHealthSummary{
					Status:    "disabled",
					UpdatedAt: now,
				},
				Membership: MeshMembershipState{
					UpdatedAt: now,
				},
				GeneratedAt: now,
			},
		}, nil
	}

	paths := m.foundationPaths(layout, runtimeCtx)
	for _, dir := range []string{layout.Mesh, paths.liveRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return MeshFoundationResult{}, err
		}
	}

	serviceCache, routes, peers := buildMeshFoundationData(runtimeCtx, now)
	membership := buildMeshMembership(runtimeCtx, peers, now)
	health := buildMeshHealthSummary(membership, routes, now)
	snapshot := MeshFoundationSnapshot{
		Enabled:     true,
		ProjectID:   runtimeCtx.Project.ProjectID,
		BindingID:   runtimeCtx.Binding.BindingID,
		RuntimeMode: runtimeCtx.Binding.RuntimeMode,
		Provider:    contracts.MeshProviderWireGuard,
		Membership:  membership,
		RouteCache:  routes,
		Health:      health,
		GeneratedAt: now,
	}

	if err := writeJSON(paths.workspaceStatePath, snapshot); err != nil {
		return MeshFoundationResult{}, err
	}
	if err := writeJSON(paths.workspaceServiceCachePath, serviceCache); err != nil {
		return MeshFoundationResult{}, err
	}
	if err := writeJSON(paths.liveStatePath, snapshot); err != nil {
		return MeshFoundationResult{}, err
	}
	if err := writeJSON(paths.liveServiceCachePath, serviceCache); err != nil {
		return MeshFoundationResult{}, err
	}

	if m.logger != nil {
		m.logger.Info("built distributed mesh foundation",
			"binding_id", runtimeCtx.Binding.BindingID,
			"local_peer_ref", snapshot.Membership.LocalPeerRef,
			"peers", len(snapshot.Membership.Peers),
			"routes", len(snapshot.RouteCache),
			"status", snapshot.Health.Status,
		)
	}

	return MeshFoundationResult{
		Snapshot:                  snapshot,
		WorkspaceStatePath:        paths.workspaceStatePath,
		WorkspaceServiceCachePath: paths.workspaceServiceCachePath,
		LiveStatePath:             paths.liveStatePath,
		LiveServiceCachePath:      paths.liveServiceCachePath,
	}, nil
}

type meshFoundationPaths struct {
	liveRoot                  string
	workspaceStatePath        string
	workspaceServiceCachePath string
	liveStatePath             string
	liveServiceCachePath      string
}

func (m *MeshManager) foundationPaths(layout WorkspaceLayout, runtimeCtx RuntimeContext) meshFoundationPaths {
	liveRoot := filepath.Join(
		m.runtimeRoot,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
		"mesh",
		"live",
	)
	return meshFoundationPaths{
		liveRoot:                  liveRoot,
		workspaceStatePath:        filepath.Join(layout.Mesh, "state.json"),
		workspaceServiceCachePath: filepath.Join(layout.Mesh, "service-metadata.json"),
		liveStatePath:             filepath.Join(liveRoot, "state.json"),
		liveServiceCachePath:      filepath.Join(liveRoot, "service-metadata.json"),
	}
}

func buildMeshFoundationData(runtimeCtx RuntimeContext, now time.Time) (ServiceMetadataCache, []MeshRouteRecord, map[string]*MeshPeer) {
	placementByService := make(map[string]contracts.PlacementAssignment, len(runtimeCtx.Revision.PlacementAssignments))
	for _, assignment := range runtimeCtx.Revision.PlacementAssignments {
		placementByService[assignment.ServiceName] = assignment
	}

	localTargetID := firstNonEmpty(runtimeCtx.Binding.TargetID, runtimeCtx.Binding.TargetRef, runtimeCtx.Project.ProjectID)
	localTargetKind := runtimeCtx.Binding.TargetKind
	localPeerRef := meshPeerRef(localTargetKind, localTargetID)
	peers := map[string]*MeshPeer{
		localPeerRef: {
			PeerRef:    localPeerRef,
			TargetID:   localTargetID,
			TargetKind: localTargetKind,
			Local:      true,
			State:      MeshPeerStateActive,
			UpdatedAt:  now,
		},
	}

	serviceCache := ServiceMetadataCache{
		Version:   meshMetadataVersion(runtimeCtx),
		UpdatedAt: now,
		Services:  make(map[string]ServiceMetadataRecord, len(runtimeCtx.Services)),
	}
	routes := make([]MeshRouteRecord, 0)

	for _, service := range runtimeCtx.Services {
		servicePlacement := resolvedPlacement(service, runtimeCtx.Binding, placementByService)
		servicePeerRef := meshPeerRef(servicePlacement.TargetKind, servicePlacement.TargetID)
		serviceScope := "local"
		if servicePeerRef != localPeerRef {
			serviceScope = "mesh"
		}

		peer := ensurePeer(peers, servicePlacement.TargetKind, servicePlacement.TargetID, servicePeerRef, now)
		peer.Services = appendUnique(peer.Services, service.Name)

		record := ServiceMetadataRecord{
			ServiceName:         service.Name,
			Public:              service.Public,
			PlacementTargetID:   servicePlacement.TargetID,
			PlacementTargetKind: servicePlacement.TargetKind,
			PlacementPeerRef:    servicePeerRef,
			RouteScope:          serviceScope,
		}

		for _, dependency := range service.Dependencies {
			targetPlacement := resolvedPlacement(findServiceByName(runtimeCtx.Services, dependency.TargetService), runtimeCtx.Binding, placementByService)
			targetPeerRef := meshPeerRef(targetPlacement.TargetKind, targetPlacement.TargetID)
			pathKind := "local"
			status := "active"
			privateOnly := false
			if targetPeerRef != servicePeerRef {
				pathKind = "mesh_private"
				status = string(MeshPeerStatePlanned)
				privateOnly = true
			}

			targetPeer := ensurePeer(peers, targetPlacement.TargetKind, targetPlacement.TargetID, targetPeerRef, now)
			targetPeer.Services = appendUnique(targetPeer.Services, dependency.TargetService)

			record.Dependencies = append(record.Dependencies, ServiceDependencyMetadata{
				Alias:            dependency.Alias,
				TargetService:    dependency.TargetService,
				Protocol:         dependency.Protocol,
				LocalEndpoint:    dependency.LocalEndpoint,
				SourceTargetID:   servicePlacement.TargetID,
				SourceTargetKind: servicePlacement.TargetKind,
				SourcePeerRef:    servicePeerRef,
				TargetTargetID:   targetPlacement.TargetID,
				TargetTargetKind: targetPlacement.TargetKind,
				TargetPeerRef:    targetPeerRef,
				RouteScope:       pathKind,
				PrivateOnly:      privateOnly,
			})

			routes = append(routes, MeshRouteRecord{
				RouteKey:       meshRouteKey(service.Name, dependency),
				ServiceName:    service.Name,
				Alias:          dependency.Alias,
				TargetService:  dependency.TargetService,
				Protocol:       dependency.Protocol,
				SourceTargetID: servicePlacement.TargetID,
				TargetTargetID: targetPlacement.TargetID,
				SourcePeerRef:  servicePeerRef,
				TargetPeerRef:  targetPeerRef,
				PathKind:       pathKind,
				Status:         status,
				PrivateOnly:    privateOnly,
				LocalEndpoint:  dependency.LocalEndpoint,
			})
		}

		sort.Slice(record.Dependencies, func(i, j int) bool {
			if record.Dependencies[i].Alias == record.Dependencies[j].Alias {
				return record.Dependencies[i].TargetService < record.Dependencies[j].TargetService
			}
			return record.Dependencies[i].Alias < record.Dependencies[j].Alias
		})
		serviceCache.Services[service.Name] = record
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].RouteKey < routes[j].RouteKey
	})
	for _, peer := range peers {
		sort.Strings(peer.Services)
	}
	return serviceCache, routes, peers
}

func buildMeshMembership(runtimeCtx RuntimeContext, peerIndex map[string]*MeshPeer, now time.Time) MeshMembershipState {
	peers := make([]MeshPeer, 0, len(peerIndex))
	localPeerRef := meshPeerRef(runtimeCtx.Binding.TargetKind, firstNonEmpty(runtimeCtx.Binding.TargetID, runtimeCtx.Binding.TargetRef, runtimeCtx.Project.ProjectID))
	for _, peer := range peerIndex {
		if peer.PeerRef == localPeerRef {
			peer.State = MeshPeerStateActive
			peer.Local = true
		} else if peer.State == "" {
			peer.State = MeshPeerStatePlanned
		}
		peers = append(peers, *peer)
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].PeerRef < peers[j].PeerRef
	})

	return MeshMembershipState{
		LocalPeerRef: localPeerRef,
		LocalState:   MeshPeerStateActive,
		Peers:        peers,
		UpdatedAt:    now,
	}
}

func buildMeshHealthSummary(membership MeshMembershipState, routes []MeshRouteRecord, now time.Time) MeshHealthSummary {
	summary := MeshHealthSummary{
		Status:      "active",
		TotalPeers:  len(membership.Peers),
		TotalRoutes: len(routes),
		UpdatedAt:   now,
	}
	for _, peer := range membership.Peers {
		switch peer.State {
		case MeshPeerStateActive:
			summary.ActivePeers++
		case MeshPeerStateDegraded:
			summary.DegradedPeers++
		default:
			summary.PlannedPeers++
		}
	}
	for _, route := range routes {
		if route.PathKind == "mesh_private" {
			summary.PrivateRoutes++
			continue
		}
		summary.LocalRoutes++
	}
	switch {
	case summary.DegradedPeers > 0:
		summary.Status = "degraded"
	case summary.PlannedPeers > 0:
		summary.Status = "planning"
	case summary.TotalPeers == 0:
		summary.Status = "disabled"
	}
	return summary
}

func meshPeerRef(targetKind contracts.TargetKind, targetID string) string {
	return fmt.Sprintf("%s:%s", targetKind, firstNonEmpty(targetID, "unknown"))
}

func meshRouteKey(serviceName string, dependency contracts.DependencyBindingPayload) string {
	base := strings.Join([]string{
		serviceName,
		dependency.Alias,
		dependency.TargetService,
		dependency.Protocol,
		dependency.LocalEndpoint,
	}, "|")
	sum := sha256.Sum256([]byte(base))
	return "route_" + hex.EncodeToString(sum[:6])
}

func meshMetadataVersion(runtimeCtx RuntimeContext) string {
	parts := []string{
		runtimeCtx.Project.ProjectID,
		runtimeCtx.Binding.BindingID,
		runtimeCtx.Revision.RevisionID,
		string(runtimeCtx.Binding.RuntimeMode),
	}
	for _, service := range runtimeCtx.Services {
		parts = append(parts, service.Name)
		if service.Placement != nil {
			parts = append(parts, fmt.Sprintf("%s|%s|%s", service.Placement.ServiceName, service.Placement.TargetKind, service.Placement.TargetID))
		}
		for _, dependency := range service.Dependencies {
			parts = append(parts, fmt.Sprintf("%s|%s|%s|%s|%s", service.Name, dependency.Alias, dependency.TargetService, dependency.Protocol, dependency.LocalEndpoint))
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "||")))
	return "meshmeta_" + hex.EncodeToString(sum[:8])
}

func ensurePeer(peers map[string]*MeshPeer, targetKind contracts.TargetKind, targetID, peerRef string, now time.Time) *MeshPeer {
	if peer, ok := peers[peerRef]; ok {
		return peer
	}
	peer := &MeshPeer{
		PeerRef:    peerRef,
		TargetID:   targetID,
		TargetKind: targetKind,
		State:      MeshPeerStatePlanned,
		UpdatedAt:  now,
	}
	peers[peerRef] = peer
	return peer
}

func resolvedPlacement(service ServiceRuntimeContext, binding contracts.DeploymentBindingPayload, placements map[string]contracts.PlacementAssignment) contracts.PlacementAssignment {
	if placement, ok := placements[service.Name]; ok {
		return placement
	}
	if service.Placement != nil {
		return contracts.PlacementAssignment{
			ServiceName: service.Placement.ServiceName,
			TargetID:    service.Placement.TargetID,
			TargetKind:  service.Placement.TargetKind,
			Labels:      service.Placement.Labels,
		}
	}
	return contracts.PlacementAssignment{
		ServiceName: service.Name,
		TargetID:    binding.TargetID,
		TargetKind:  binding.TargetKind,
	}
}

func findServiceByName(services []ServiceRuntimeContext, name string) ServiceRuntimeContext {
	for _, service := range services {
		if service.Name == name {
			return service
		}
	}
	return ServiceRuntimeContext{Name: name}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
