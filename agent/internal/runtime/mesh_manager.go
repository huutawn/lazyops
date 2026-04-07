package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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
	RouteKey              string                 `json:"route_key"`
	ServiceName           string                 `json:"service_name"`
	Alias                 string                 `json:"alias"`
	TargetService         string                 `json:"target_service"`
	Protocol              string                 `json:"protocol"`
	SourceTargetID        string                 `json:"source_target_id"`
	TargetTargetID        string                 `json:"target_target_id"`
	SourcePeerRef         string                 `json:"source_peer_ref"`
	TargetPeerRef         string                 `json:"target_peer_ref"`
	PathKind              string                 `json:"path_kind"`
	Status                string                 `json:"status"`
	Provider              contracts.MeshProvider `json:"provider,omitempty"`
	Verified              bool                   `json:"verified"`
	PrivateOnly           bool                   `json:"private_only"`
	PublicFallbackBlocked bool                   `json:"public_fallback_blocked,omitempty"`
	Summary               string                 `json:"summary,omitempty"`
	LocalEndpoint         string                 `json:"local_endpoint,omitempty"`
}

type MeshHealthSummary struct {
	Status         string    `json:"status"`
	TotalPeers     int       `json:"total_peers"`
	ActivePeers    int       `json:"active_peers"`
	PlannedPeers   int       `json:"planned_peers"`
	DegradedPeers  int       `json:"degraded_peers"`
	TotalRoutes    int       `json:"total_routes"`
	LocalRoutes    int       `json:"local_routes"`
	PrivateRoutes  int       `json:"private_routes"`
	VerifiedRoutes int       `json:"verified_routes"`
	DegradedRoutes int       `json:"degraded_routes"`
	BlockedRoutes  int       `json:"blocked_routes"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ServiceMetadataCache struct {
	Version              string                           `json:"version"`
	UpdatedAt            time.Time                        `json:"updated_at"`
	PlacementFingerprint string                           `json:"placement_fingerprint,omitempty"`
	RouteFingerprint     string                           `json:"route_fingerprint,omitempty"`
	InvalidationRules    []string                         `json:"invalidation_rules,omitempty"`
	Services             map[string]ServiceMetadataRecord `json:"services"`
}

type ServiceMetadataRecord struct {
	ServiceName            string                      `json:"service_name"`
	Public                 bool                        `json:"public"`
	PlacementTargetID      string                      `json:"placement_target_id"`
	PlacementTargetKind    contracts.TargetKind        `json:"placement_target_kind"`
	PlacementPeerRef       string                      `json:"placement_peer_ref"`
	RouteScope             string                      `json:"route_scope"`
	CacheInvalidationRules []string                    `json:"cache_invalidation_rules,omitempty"`
	Dependencies           []ServiceDependencyMetadata `json:"dependencies,omitempty"`
}

type ServiceDependencyMetadata struct {
	Alias                 string                 `json:"alias"`
	TargetService         string                 `json:"target_service"`
	Protocol              string                 `json:"protocol"`
	LocalEndpoint         string                 `json:"local_endpoint,omitempty"`
	SourceTargetID        string                 `json:"source_target_id"`
	SourceTargetKind      contracts.TargetKind   `json:"source_target_kind"`
	SourcePeerRef         string                 `json:"source_peer_ref"`
	TargetTargetID        string                 `json:"target_target_id"`
	TargetTargetKind      contracts.TargetKind   `json:"target_target_kind"`
	TargetPeerRef         string                 `json:"target_peer_ref"`
	RouteScope            string                 `json:"route_scope"`
	RouteStatus           string                 `json:"route_status,omitempty"`
	Provider              contracts.MeshProvider `json:"provider,omitempty"`
	PrivateOnly           bool                   `json:"private_only"`
	PublicFallbackBlocked bool                   `json:"public_fallback_blocked,omitempty"`
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

type MeshCapabilitySignals struct {
	Provider             contracts.MeshProvider   `json:"provider"`
	ActiveProvider       contracts.MeshProvider   `json:"active_provider,omitempty"`
	SupportedProviders   []contracts.MeshProvider `json:"supported_providers,omitempty"`
	ReservedProviders    []contracts.MeshProvider `json:"reserved_providers,omitempty"`
	DeterministicCleanup bool                     `json:"deterministic_cleanup"`
	PrivateOverlay       bool                     `json:"private_overlay"`
	CrossNodePrivateOnly bool                     `json:"cross_node_private_only"`
	RecordedAt           time.Time                `json:"recorded_at"`
}

type MeshHealthSignals struct {
	Provider       contracts.MeshProvider `json:"provider"`
	Status         string                 `json:"status"`
	ActivePeers    int                    `json:"active_peers"`
	PlannedPeers   int                    `json:"planned_peers"`
	DegradedPeers  int                    `json:"degraded_peers"`
	VerifiedRoutes int                    `json:"verified_routes,omitempty"`
	DegradedRoutes int                    `json:"degraded_routes,omitempty"`
	BlockedRoutes  int                    `json:"blocked_routes,omitempty"`
	EnsuredPeers   []string               `json:"ensured_peers,omitempty"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type WireGuardPeerRecord struct {
	PeerRef                 string               `json:"peer_ref"`
	TargetID                string               `json:"target_id"`
	TargetKind              contracts.TargetKind `json:"target_kind"`
	Address                 string               `json:"address"`
	ListenPort              int                  `json:"listen_port"`
	PublicKey               string               `json:"public_key"`
	PrivateKeyFingerprint   string               `json:"private_key_fingerprint"`
	PresharedKeyFingerprint string               `json:"preshared_key_fingerprint"`
	ConfigPath              string               `json:"config_path"`
	LastAppliedAt           time.Time            `json:"last_applied_at"`
}

type EnsureMeshPeerResult struct {
	Provider              contracts.MeshProvider `json:"provider"`
	EnsuredPeerRefs       []string               `json:"ensured_peer_refs,omitempty"`
	RemovedPeerRefs       []string               `json:"removed_peer_refs,omitempty"`
	StatePath             string                 `json:"state_path"`
	ServiceCachePath      string                 `json:"service_cache_path"`
	HealthSignalsPath     string                 `json:"health_signals_path"`
	CapabilitySignalsPath string                 `json:"capability_signals_path"`
	ProviderRoot          string                 `json:"provider_root"`
	Summary               string                 `json:"summary"`
}

type MeshLinkHealthRecord struct {
	LinkKey       string                 `json:"link_key"`
	SourcePeerRef string                 `json:"source_peer_ref"`
	TargetPeerRef string                 `json:"target_peer_ref"`
	Provider      contracts.MeshProvider `json:"provider"`
	Status        string                 `json:"status"`
	Verified      bool                   `json:"verified"`
	PrivateOnly   bool                   `json:"private_only"`
	RouteCount    int                    `json:"route_count"`
	Summary       string                 `json:"summary"`
}

type MeshAdapterSlot struct {
	Provider   contracts.MeshProvider `json:"provider"`
	Status     string                 `json:"status"`
	Reserved   bool                   `json:"reserved"`
	Active     bool                   `json:"active"`
	ConfigRoot string                 `json:"config_root,omitempty"`
	Summary    string                 `json:"summary"`
}

type SyncOverlayRoutesResult struct {
	Provider              contracts.MeshProvider `json:"provider"`
	StatePath             string                 `json:"state_path"`
	ServiceCachePath      string                 `json:"service_cache_path"`
	RouteReportPath       string                 `json:"route_report_path"`
	LinkHealthPath        string                 `json:"link_health_path"`
	AdapterSlotsPath      string                 `json:"adapter_slots_path"`
	HealthSignalsPath     string                 `json:"health_signals_path"`
	CapabilitySignalsPath string                 `json:"capability_signals_path"`
	VerifiedRoutes        int                    `json:"verified_routes"`
	DegradedRoutes        int                    `json:"degraded_routes"`
	BlockedRoutes         int                    `json:"blocked_routes"`
	Summary               string                 `json:"summary"`
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
	routeReportPath           string
	linkHealthPath            string
	adapterSlotsPath          string
	healthSignalsPath         string
	capabilitySignalsPath     string
	providerRoot              string
	tailscaleRoot             string
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
		routeReportPath:           filepath.Join(liveRoot, "overlay-routes.json"),
		linkHealthPath:            filepath.Join(liveRoot, "link-health.json"),
		adapterSlotsPath:          filepath.Join(liveRoot, "adapter-slots.json"),
		healthSignalsPath:         filepath.Join(liveRoot, "health-signals.json"),
		capabilitySignalsPath:     filepath.Join(liveRoot, "capability-signals.json"),
		providerRoot:              filepath.Join(liveRoot, "wireguard"),
		tailscaleRoot:             filepath.Join(liveRoot, "tailscale"),
	}
}

func (m *MeshManager) EnsurePeer(_ context.Context, payload contracts.EnsureMeshPeerPayload) (EnsureMeshPeerResult, error) {
	if strings.TrimSpace(payload.ProjectID) == "" {
		return EnsureMeshPeerResult{}, &OperationError{
			Code:      "mesh_missing_project_id",
			Message:   "ensure_mesh_peer requires project_id",
			Retryable: false,
		}
	}
	if strings.TrimSpace(payload.BindingID) == "" {
		return EnsureMeshPeerResult{}, &OperationError{
			Code:      "mesh_missing_binding_id",
			Message:   "ensure_mesh_peer requires binding_id",
			Retryable: false,
		}
	}
	if payload.RuntimeMode != contracts.RuntimeModeDistributedMesh {
		return EnsureMeshPeerResult{}, &OperationError{
			Code:      "mesh_runtime_mode_disabled",
			Message:   fmt.Sprintf("mesh manager is disabled for runtime mode %q", payload.RuntimeMode),
			Retryable: false,
		}
	}
	provider := payload.Provider
	if provider == "" {
		provider = contracts.MeshProviderWireGuard
	}
	if provider != contracts.MeshProviderWireGuard {
		return EnsureMeshPeerResult{}, &OperationError{
			Code:      "mesh_provider_not_supported",
			Message:   fmt.Sprintf("mesh provider %q is not supported yet", provider),
			Retryable: false,
		}
	}

	paths := m.livePaths(payload.ProjectID, payload.BindingID)
	snapshot, err := loadMeshFoundationSnapshot(paths.liveStatePath)
	if err != nil {
		return EnsureMeshPeerResult{}, &OperationError{
			Code:      "mesh_live_state_missing",
			Message:   "mesh live state is required before peers can be ensured",
			Retryable: true,
			Err:       err,
		}
	}
	serviceCache, err := loadServiceMetadataCache(paths.liveServiceCachePath)
	if err != nil {
		return EnsureMeshPeerResult{}, &OperationError{
			Code:      "mesh_service_cache_missing",
			Message:   "mesh service metadata cache is required before peers can be ensured",
			Retryable: true,
			Err:       err,
		}
	}

	desiredState := strings.TrimSpace(payload.DesiredState)
	if desiredState == "" {
		desiredState = string(MeshPeerStateActive)
	}
	if !validMeshPeerState(MeshPeerState(desiredState)) {
		return EnsureMeshPeerResult{}, &OperationError{
			Code:      "mesh_invalid_desired_state",
			Message:   fmt.Sprintf("mesh desired state %q is invalid", desiredState),
			Retryable: false,
		}
	}

	if err := os.MkdirAll(paths.providerRoot, 0o700); err != nil {
		return EnsureMeshPeerResult{}, err
	}

	ensuredPeerRefs := make([]string, 0)
	removedPeerRefs := make([]string, 0)
	now := m.now()
	for index := range snapshot.Membership.Peers {
		peer := &snapshot.Membership.Peers[index]
		if peer.Local {
			continue
		}
		if payload.PeerRef != "" && payload.PeerRef != peer.PeerRef {
			continue
		}
		if payload.TargetID != "" && payload.TargetID != peer.TargetID {
			continue
		}
		if payload.TargetKind != "" && payload.TargetKind != peer.TargetKind {
			continue
		}

		switch MeshPeerState(desiredState) {
		case MeshPeerStateLeaving, MeshPeerStateRemoved:
			peer.State = MeshPeerStateLeaving
			if err := removeWireGuardPeer(paths.providerRoot, peer.PeerRef); err != nil {
				return EnsureMeshPeerResult{}, err
			}
			peer.State = MeshPeerStateRemoved
			peer.UpdatedAt = now
			removedPeerRefs = append(removedPeerRefs, peer.PeerRef)
		default:
			peer.State = MeshPeerStateJoining
			record, err := ensureWireGuardPeer(paths.providerRoot, *peer, now)
			if err != nil {
				return EnsureMeshPeerResult{}, err
			}
			peer.State = MeshPeerStateActive
			peer.UpdatedAt = now
			ensuredPeerRefs = append(ensuredPeerRefs, peer.PeerRef)
			if m.logger != nil {
				m.logger.Info("wireguard peer ensured",
					"peer_ref", peer.PeerRef,
					"target_id", peer.TargetID,
					"address", record.Address,
				)
			}
		}
	}

	snapshot.Health = buildMeshHealthSummary(snapshot.Membership, snapshot.RouteCache, now)
	snapshot.GeneratedAt = now

	healthSignals := MeshHealthSignals{
		Provider:      provider,
		Status:        snapshot.Health.Status,
		ActivePeers:   snapshot.Health.ActivePeers,
		PlannedPeers:  snapshot.Health.PlannedPeers,
		DegradedPeers: snapshot.Health.DegradedPeers,
		EnsuredPeers:  append([]string(nil), ensuredPeerRefs...),
		UpdatedAt:     now,
	}
	capabilitySignals := MeshCapabilitySignals{
		Provider:             provider,
		ActiveProvider:       contracts.MeshProviderWireGuard,
		SupportedProviders:   []contracts.MeshProvider{contracts.MeshProviderWireGuard, contracts.MeshProviderTailscale},
		ReservedProviders:    []contracts.MeshProvider{contracts.MeshProviderTailscale},
		DeterministicCleanup: true,
		PrivateOverlay:       true,
		CrossNodePrivateOnly: true,
		RecordedAt:           now,
	}

	if err := writeJSON(paths.liveStatePath, snapshot); err != nil {
		return EnsureMeshPeerResult{}, err
	}
	if err := writeJSON(paths.liveServiceCachePath, serviceCache); err != nil {
		return EnsureMeshPeerResult{}, err
	}
	if err := writeJSON(paths.healthSignalsPath, healthSignals); err != nil {
		return EnsureMeshPeerResult{}, err
	}
	if err := writeJSON(paths.capabilitySignalsPath, capabilitySignals); err != nil {
		return EnsureMeshPeerResult{}, err
	}

	summary := meshEnsureSummary(ensuredPeerRefs, removedPeerRefs, snapshot.Health.Status)
	return EnsureMeshPeerResult{
		Provider:              provider,
		EnsuredPeerRefs:       ensuredPeerRefs,
		RemovedPeerRefs:       removedPeerRefs,
		StatePath:             paths.liveStatePath,
		ServiceCachePath:      paths.liveServiceCachePath,
		HealthSignalsPath:     paths.healthSignalsPath,
		CapabilitySignalsPath: paths.capabilitySignalsPath,
		ProviderRoot:          paths.providerRoot,
		Summary:               summary,
	}, nil
}

func (m *MeshManager) livePaths(projectID, bindingID string) meshFoundationPaths {
	liveRoot := filepath.Join(
		m.runtimeRoot,
		"projects",
		projectID,
		"bindings",
		bindingID,
		"mesh",
		"live",
	)
	return meshFoundationPaths{
		liveRoot:              liveRoot,
		liveStatePath:         filepath.Join(liveRoot, "state.json"),
		liveServiceCachePath:  filepath.Join(liveRoot, "service-metadata.json"),
		routeReportPath:       filepath.Join(liveRoot, "overlay-routes.json"),
		linkHealthPath:        filepath.Join(liveRoot, "link-health.json"),
		adapterSlotsPath:      filepath.Join(liveRoot, "adapter-slots.json"),
		healthSignalsPath:     filepath.Join(liveRoot, "health-signals.json"),
		capabilitySignalsPath: filepath.Join(liveRoot, "capability-signals.json"),
		providerRoot:          filepath.Join(liveRoot, "wireguard"),
		tailscaleRoot:         filepath.Join(liveRoot, "tailscale"),
	}
}

func (m *MeshManager) SyncOverlayRoutes(_ context.Context, payload contracts.SyncOverlayRoutesPayload) (SyncOverlayRoutesResult, error) {
	if strings.TrimSpace(payload.ProjectID) == "" {
		return SyncOverlayRoutesResult{}, &OperationError{
			Code:      "mesh_missing_project_id",
			Message:   "sync_overlay_routes requires project_id",
			Retryable: false,
		}
	}
	if strings.TrimSpace(payload.BindingID) == "" {
		return SyncOverlayRoutesResult{}, &OperationError{
			Code:      "mesh_missing_binding_id",
			Message:   "sync_overlay_routes requires binding_id",
			Retryable: false,
		}
	}
	if payload.RuntimeMode != contracts.RuntimeModeDistributedMesh {
		return SyncOverlayRoutesResult{}, &OperationError{
			Code:      "mesh_runtime_mode_disabled",
			Message:   fmt.Sprintf("mesh route sync is disabled for runtime mode %q", payload.RuntimeMode),
			Retryable: false,
		}
	}

	provider := payload.Provider
	if provider == "" {
		provider = contracts.MeshProviderWireGuard
	}
	if provider == contracts.MeshProviderTailscale {
		return SyncOverlayRoutesResult{}, &OperationError{
			Code:      "mesh_provider_reserved",
			Message:   "tailscale adapter slot is reserved but not active yet",
			Retryable: false,
		}
	}
	if provider != contracts.MeshProviderWireGuard {
		return SyncOverlayRoutesResult{}, &OperationError{
			Code:      "mesh_provider_not_supported",
			Message:   fmt.Sprintf("mesh provider %q is not supported yet", provider),
			Retryable: false,
		}
	}

	paths := m.livePaths(payload.ProjectID, payload.BindingID)
	snapshot, err := loadMeshFoundationSnapshot(paths.liveStatePath)
	if err != nil {
		return SyncOverlayRoutesResult{}, &OperationError{
			Code:      "mesh_live_state_missing",
			Message:   "mesh live state is required before overlay routes can be synchronized",
			Retryable: true,
			Err:       err,
		}
	}
	serviceCache, err := loadServiceMetadataCache(paths.liveServiceCachePath)
	if err != nil {
		return SyncOverlayRoutesResult{}, &OperationError{
			Code:      "mesh_service_cache_missing",
			Message:   "mesh service metadata cache is required before overlay routes can be synchronized",
			Retryable: true,
			Err:       err,
		}
	}

	for _, dir := range []string{paths.liveRoot, paths.providerRoot, paths.tailscaleRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return SyncOverlayRoutesResult{}, err
		}
	}

	now := m.now()
	peerIndex := make(map[string]MeshPeer, len(snapshot.Membership.Peers))
	for _, peer := range snapshot.Membership.Peers {
		peerIndex[peer.PeerRef] = peer
	}

	routeReport := make([]MeshRouteRecord, len(snapshot.RouteCache))
	copy(routeReport, snapshot.RouteCache)

	linkHealthMap := make(map[string]*MeshLinkHealthRecord)
	for index := range routeReport {
		route := &routeReport[index]
		if route.PathKind != "mesh_private" {
			route.Status = "local_verified"
			route.Verified = true
			route.Provider = ""
			route.PublicFallbackBlocked = false
			route.Summary = "route stays on the local target and does not require overlay transport"
			syncServiceCacheRoute(&serviceCache, *route)
			continue
		}

		sourcePeer := peerIndex[route.SourcePeerRef]
		targetPeer, ok := peerIndex[route.TargetPeerRef]
		route.Provider = provider
		route.PrivateOnly = true
		route.PublicFallbackBlocked = true
		route.Verified = false
		switch {
		case !ok || targetPeer.State == MeshPeerStateRemoved || targetPeer.State == MeshPeerStateLeaving:
			route.Status = "blocked"
			route.Summary = fmt.Sprintf("private overlay route to %s is unavailable because the target peer is not present", route.TargetPeerRef)
		case sourcePeer.State == MeshPeerStateDegraded || targetPeer.State == MeshPeerStateDegraded:
			route.Status = "degraded"
			route.Summary = fmt.Sprintf("private overlay route to %s is degraded because one or more peers are degraded", route.TargetPeerRef)
		case sourcePeer.State == MeshPeerStateActive && targetPeer.State == MeshPeerStateActive:
			route.Status = "verified"
			route.Verified = true
			route.Summary = fmt.Sprintf("private overlay route to %s is verified through %s", route.TargetPeerRef, provider)
		default:
			route.Status = "blocked"
			route.Summary = fmt.Sprintf("private overlay route to %s is blocked until peer reconciliation completes", route.TargetPeerRef)
		}

		link := ensureLinkHealthRecord(linkHealthMap, route, provider)
		link.RouteCount++
		link.PrivateOnly = true
		mergeLinkHealthStatus(link, route)
		syncServiceCacheRoute(&serviceCache, *route)
	}

	linkHealth := flattenLinkHealth(linkHealthMap)
	adapterSlots := buildMeshAdapterSlots(paths, provider)
	snapshot.RouteCache = routeReport
	snapshot.Health = buildMeshHealthSummary(snapshot.Membership, snapshot.RouteCache, now)
	snapshot.GeneratedAt = now
	serviceCache.UpdatedAt = now
	serviceCache.RouteFingerprint = meshRouteFingerprint(routeReport)
	if len(serviceCache.InvalidationRules) == 0 {
		serviceCache.InvalidationRules = resolutionInvalidationRules()
	}
	for serviceName, record := range serviceCache.Services {
		record.CacheInvalidationRules = append([]string(nil), serviceCache.InvalidationRules...)
		serviceCache.Services[serviceName] = record
	}

	healthSignals := MeshHealthSignals{
		Provider:       provider,
		Status:         snapshot.Health.Status,
		ActivePeers:    snapshot.Health.ActivePeers,
		PlannedPeers:   snapshot.Health.PlannedPeers,
		DegradedPeers:  snapshot.Health.DegradedPeers,
		VerifiedRoutes: snapshot.Health.VerifiedRoutes,
		DegradedRoutes: snapshot.Health.DegradedRoutes,
		BlockedRoutes:  snapshot.Health.BlockedRoutes,
		UpdatedAt:      now,
	}
	capabilitySignals := MeshCapabilitySignals{
		Provider:             provider,
		ActiveProvider:       contracts.MeshProviderWireGuard,
		SupportedProviders:   []contracts.MeshProvider{contracts.MeshProviderWireGuard, contracts.MeshProviderTailscale},
		ReservedProviders:    []contracts.MeshProvider{contracts.MeshProviderTailscale},
		DeterministicCleanup: true,
		PrivateOverlay:       true,
		CrossNodePrivateOnly: true,
		RecordedAt:           now,
	}

	if err := writeJSON(paths.liveStatePath, snapshot); err != nil {
		return SyncOverlayRoutesResult{}, err
	}
	if err := writeJSON(paths.liveServiceCachePath, serviceCache); err != nil {
		return SyncOverlayRoutesResult{}, err
	}
	if err := writeJSON(paths.routeReportPath, routeReport); err != nil {
		return SyncOverlayRoutesResult{}, err
	}
	if err := writeJSON(paths.linkHealthPath, linkHealth); err != nil {
		return SyncOverlayRoutesResult{}, err
	}
	if err := writeJSON(paths.adapterSlotsPath, adapterSlots); err != nil {
		return SyncOverlayRoutesResult{}, err
	}
	if err := writeJSON(filepath.Join(paths.tailscaleRoot, "slot.json"), adapterSlots[1]); err != nil {
		return SyncOverlayRoutesResult{}, err
	}
	if err := writeJSON(paths.healthSignalsPath, healthSignals); err != nil {
		return SyncOverlayRoutesResult{}, err
	}
	if err := writeJSON(paths.capabilitySignalsPath, capabilitySignals); err != nil {
		return SyncOverlayRoutesResult{}, err
	}

	summary := fmt.Sprintf(
		"overlay route sync verified %d route(s), degraded %d route(s), and blocked %d route(s)",
		snapshot.Health.VerifiedRoutes,
		snapshot.Health.DegradedRoutes,
		snapshot.Health.BlockedRoutes,
	)
	if m.logger != nil {
		m.logger.Info("overlay routes synchronized",
			"binding_id", payload.BindingID,
			"provider", provider,
			"verified_routes", snapshot.Health.VerifiedRoutes,
			"degraded_routes", snapshot.Health.DegradedRoutes,
			"blocked_routes", snapshot.Health.BlockedRoutes,
		)
	}

	return SyncOverlayRoutesResult{
		Provider:              provider,
		StatePath:             paths.liveStatePath,
		ServiceCachePath:      paths.liveServiceCachePath,
		RouteReportPath:       paths.routeReportPath,
		LinkHealthPath:        paths.linkHealthPath,
		AdapterSlotsPath:      paths.adapterSlotsPath,
		HealthSignalsPath:     paths.healthSignalsPath,
		CapabilitySignalsPath: paths.capabilitySignalsPath,
		VerifiedRoutes:        snapshot.Health.VerifiedRoutes,
		DegradedRoutes:        snapshot.Health.DegradedRoutes,
		BlockedRoutes:         snapshot.Health.BlockedRoutes,
		Summary:               summary,
	}, nil
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
		Version:              meshMetadataVersion(runtimeCtx),
		UpdatedAt:            now,
		PlacementFingerprint: runtimeCtx.Runtime.PlacementFingerprint,
		InvalidationRules:    resolutionInvalidationRules(),
		Services:             make(map[string]ServiceMetadataRecord, len(runtimeCtx.Services)),
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
			ServiceName:            service.Name,
			Public:                 service.Public,
			PlacementTargetID:      servicePlacement.TargetID,
			PlacementTargetKind:    servicePlacement.TargetKind,
			PlacementPeerRef:       servicePeerRef,
			RouteScope:             serviceScope,
			CacheInvalidationRules: append([]string(nil), serviceCache.InvalidationRules...),
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
				Alias:                 dependency.Alias,
				TargetService:         dependency.TargetService,
				Protocol:              dependency.Protocol,
				LocalEndpoint:         dependency.LocalEndpoint,
				SourceTargetID:        servicePlacement.TargetID,
				SourceTargetKind:      servicePlacement.TargetKind,
				SourcePeerRef:         servicePeerRef,
				TargetTargetID:        targetPlacement.TargetID,
				TargetTargetKind:      targetPlacement.TargetKind,
				TargetPeerRef:         targetPeerRef,
				RouteScope:            pathKind,
				RouteStatus:           status,
				PrivateOnly:           privateOnly,
				PublicFallbackBlocked: privateOnly,
			})

			routes = append(routes, MeshRouteRecord{
				RouteKey:              meshRouteKey(service.Name, dependency),
				ServiceName:           service.Name,
				Alias:                 dependency.Alias,
				TargetService:         dependency.TargetService,
				Protocol:              dependency.Protocol,
				SourceTargetID:        servicePlacement.TargetID,
				TargetTargetID:        targetPlacement.TargetID,
				SourcePeerRef:         servicePeerRef,
				TargetPeerRef:         targetPeerRef,
				PathKind:              pathKind,
				Status:                status,
				PrivateOnly:           privateOnly,
				PublicFallbackBlocked: privateOnly,
				Summary:               initialRouteSummary(pathKind, status, targetPeerRef),
				LocalEndpoint:         dependency.LocalEndpoint,
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
	serviceCache.RouteFingerprint = meshRouteFingerprint(routes)
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
		} else {
			summary.LocalRoutes++
		}
		switch route.Status {
		case "verified", "local_verified", "active":
			summary.VerifiedRoutes++
		case "degraded":
			summary.DegradedRoutes++
		case "blocked", "unavailable":
			summary.BlockedRoutes++
		}
	}
	switch {
	case summary.BlockedRoutes > 0 || summary.DegradedRoutes > 0 || summary.DegradedPeers > 0:
		summary.Status = "degraded"
	case summary.PlannedPeers > 0:
		summary.Status = "planning"
	case summary.TotalPeers == 0:
		summary.Status = "disabled"
	}
	return summary
}

func initialRouteSummary(pathKind, status, targetPeerRef string) string {
	switch pathKind {
	case "mesh_private":
		return fmt.Sprintf("private overlay route to %s is %s until peer reconciliation verifies it", targetPeerRef, status)
	default:
		return "local route remains on the same target"
	}
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

func loadMeshFoundationSnapshot(path string) (MeshFoundationSnapshot, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return MeshFoundationSnapshot{}, err
	}
	var snapshot MeshFoundationSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return MeshFoundationSnapshot{}, err
	}
	return snapshot, nil
}

func loadServiceMetadataCache(path string) (ServiceMetadataCache, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ServiceMetadataCache{}, err
	}
	var cache ServiceMetadataCache
	if err := json.Unmarshal(payload, &cache); err != nil {
		return ServiceMetadataCache{}, err
	}
	return cache, nil
}

func validMeshPeerState(state MeshPeerState) bool {
	switch state {
	case MeshPeerStatePlanned, MeshPeerStateJoining, MeshPeerStateActive, MeshPeerStateDegraded, MeshPeerStateLeaving, MeshPeerStateRemoved:
		return true
	default:
		return false
	}
}

func ensureWireGuardPeer(providerRoot string, peer MeshPeer, now time.Time) (WireGuardPeerRecord, error) {
	peerDir := filepath.Join(providerRoot, sanitizePathToken(peer.PeerRef))
	if err := os.MkdirAll(peerDir, 0o700); err != nil {
		return WireGuardPeerRecord{}, err
	}

	privateKeyPath := filepath.Join(peerDir, "private.key")
	presharedKeyPath := filepath.Join(peerDir, "preshared.key")
	publicKeyPath := filepath.Join(peerDir, "public.key")
	configPath := filepath.Join(peerDir, "wg.conf")
	recordPath := filepath.Join(peerDir, "peer.json")

	privateKey, err := ensureSecretFile(privateKeyPath)
	if err != nil {
		return WireGuardPeerRecord{}, err
	}
	presharedKey, err := ensureSecretFile(presharedKeyPath)
	if err != nil {
		return WireGuardPeerRecord{}, err
	}
	publicKey := wireGuardPublicKey(peer)
	if err := os.WriteFile(publicKeyPath, []byte(publicKey), 0o644); err != nil {
		return WireGuardPeerRecord{}, err
	}

	address := wireGuardAddress(peer.PeerRef)
	listenPort := wireGuardListenPort(peer.PeerRef)
	config := renderWireGuardConfig(peer, address, listenPort, privateKey, publicKey, presharedKey)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return WireGuardPeerRecord{}, err
	}

	record := WireGuardPeerRecord{
		PeerRef:                 peer.PeerRef,
		TargetID:                peer.TargetID,
		TargetKind:              peer.TargetKind,
		Address:                 address,
		ListenPort:              listenPort,
		PublicKey:               publicKey,
		PrivateKeyFingerprint:   secretFingerprint(privateKey),
		PresharedKeyFingerprint: secretFingerprint(presharedKey),
		ConfigPath:              configPath,
		LastAppliedAt:           now,
	}
	if err := writeJSON(recordPath, record); err != nil {
		return WireGuardPeerRecord{}, err
	}

	ifName := wireGuardInterfaceName(peer.PeerRef)
	if err := execWireGuardUp(configPath, ifName); err != nil {
		record.LastAppliedAt = time.Time{}
		_ = writeJSON(recordPath, record)
		return record, &OperationError{
			Code:      "wireguard_up_failed",
			Message:   fmt.Sprintf("wg-quick up failed for peer %s: %v", peer.PeerRef, err),
			Retryable: true,
			Err:       err,
		}
	}

	return record, nil
}

func removeWireGuardPeer(providerRoot, peerRef string) error {
	ifName := wireGuardInterfaceName(peerRef)
	if err := execWireGuardDown(ifName); err != nil {
		slog.Default().Warn("wg-quick down failed during peer removal, proceeding with cleanup",
			"peer_ref", peerRef,
			"interface", ifName,
			"error", err,
		)
	}
	return os.RemoveAll(filepath.Join(providerRoot, sanitizePathToken(peerRef)))
}

// wireGuardInterfaceName derives a Linux-safe interface name (max 15 chars) from a peer ref.
func wireGuardInterfaceName(peerRef string) string {
	sum := sha256.Sum256([]byte(peerRef))
	return "wg" + hex.EncodeToString(sum[:6])
}

// execWireGuardUp brings up a WireGuard interface using wg-quick.
// If the interface already exists, it is brought down first.
func execWireGuardUp(configPath, ifName string) error {
	if _, err := exec.LookPath("wg-quick"); err != nil {
		slog.Default().Warn("wg-quick binary not found, skipping WireGuard activation (dev mode)",
			"config_path", configPath,
			"interface", ifName,
		)
		return nil
	}

	// Bring down existing interface if present (ignore errors)
	down := exec.Command("wg-quick", "down", ifName)
	_ = down.Run()

	up := exec.Command("wg-quick", "up", configPath)
	up.Env = append(os.Environ(), "WG_I_PREFER_BUGGY_USERSPACE_TO_POLISHED_KMOD=1")
	output, err := up.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick up %s: %s: %w", configPath, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// execWireGuardDown brings down a WireGuard interface.
func execWireGuardDown(ifName string) error {
	if _, err := exec.LookPath("wg-quick"); err != nil {
		return nil
	}
	down := exec.Command("wg-quick", "down", ifName)
	output, err := down.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick down %s: %s: %w", ifName, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CheckWireGuardAvailable returns nil if wg and wg-quick binaries are available.
func CheckWireGuardAvailable() error {
	for _, binary := range []string{"wg", "wg-quick"} {
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("%s binary not found in PATH: %w", binary, err)
		}
	}
	return nil
}

func ensureSecretFile(path string) (string, error) {
	if payload, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(payload)), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	secret := randomMeshSecret(path)
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		return "", err
	}
	return secret, nil
}

func randomMeshSecret(seed string) string {
	sum := sha256.Sum256([]byte(seed + "|" + time.Now().UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:16])
}

func wireGuardPublicKey(peer MeshPeer) string {
	sum := sha256.Sum256([]byte("public|" + peer.PeerRef + "|" + peer.TargetID))
	return hex.EncodeToString(sum[:16])
}

func wireGuardAddress(peerRef string) string {
	sum := sha256.Sum256([]byte("address|" + peerRef))
	return fmt.Sprintf("10.%d.%d.%d/32", sum[0], sum[1], maxMeshOctet(sum[2]))
}

func wireGuardListenPort(peerRef string) int {
	sum := sha256.Sum256([]byte("port|" + peerRef))
	return 20000 + int(sum[0])<<4 + int(sum[1])%1000
}

func renderWireGuardConfig(peer MeshPeer, address string, listenPort int, privateKey, publicKey, presharedKey string) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Interface]
Address = %s
ListenPort = %d
PrivateKey = %s

[Peer]
# peer_ref = %s
PublicKey = %s
PresharedKey = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, address, listenPort, privateKey, peer.PeerRef, publicKey, presharedKey, strings.TrimSuffix(address, "/32"))) + "\n"
}

func secretFingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:8])
}

func sanitizePathToken(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_")
	return replacer.Replace(value)
}

func maxMeshOctet(value byte) int {
	if value == 0 {
		return 1
	}
	return int(value)
}

func meshEnsureSummary(ensuredPeerRefs, removedPeerRefs []string, status string) string {
	switch {
	case len(removedPeerRefs) > 0:
		return fmt.Sprintf("mesh peer cleanup applied to %d peer(s); mesh health is %s", len(removedPeerRefs), status)
	case len(ensuredPeerRefs) > 0:
		return fmt.Sprintf("mesh peer ensure applied to %d peer(s); mesh health is %s", len(ensuredPeerRefs), status)
	default:
		return fmt.Sprintf("mesh peer ensure found no matching remote peers; mesh health is %s", status)
	}
}

func syncServiceCacheRoute(cache *ServiceMetadataCache, route MeshRouteRecord) {
	if cache == nil {
		return
	}
	record, ok := cache.Services[route.ServiceName]
	if !ok {
		return
	}
	for index := range record.Dependencies {
		dependency := &record.Dependencies[index]
		if dependency.Alias != route.Alias || dependency.TargetService != route.TargetService {
			continue
		}
		dependency.RouteScope = route.PathKind
		dependency.RouteStatus = route.Status
		dependency.Provider = route.Provider
		dependency.PrivateOnly = route.PrivateOnly
		dependency.PublicFallbackBlocked = route.PublicFallbackBlocked
	}
	cache.Services[route.ServiceName] = record
}

func ensureLinkHealthRecord(records map[string]*MeshLinkHealthRecord, route *MeshRouteRecord, provider contracts.MeshProvider) *MeshLinkHealthRecord {
	linkKey := route.SourcePeerRef + "->" + route.TargetPeerRef
	if record, ok := records[linkKey]; ok {
		return record
	}
	record := &MeshLinkHealthRecord{
		LinkKey:       linkKey,
		SourcePeerRef: route.SourcePeerRef,
		TargetPeerRef: route.TargetPeerRef,
		Provider:      provider,
		Status:        "verified",
		Summary:       "overlay link is verified",
	}
	records[linkKey] = record
	return record
}

func mergeLinkHealthStatus(record *MeshLinkHealthRecord, route *MeshRouteRecord) {
	if record == nil || route == nil {
		return
	}
	switch route.Status {
	case "blocked", "unavailable":
		record.Status = "unavailable"
		record.Verified = false
		record.Summary = fmt.Sprintf("overlay link is unavailable because route %s is blocked", route.RouteKey)
	case "degraded":
		if record.Status != "unavailable" {
			record.Status = "degraded"
			record.Verified = false
			record.Summary = fmt.Sprintf("overlay link is degraded because route %s is degraded", route.RouteKey)
		}
	default:
		if record.Status == "" || record.Status == "verified" {
			record.Status = "verified"
			record.Verified = true
			record.Summary = fmt.Sprintf("overlay link is verified through provider %s", record.Provider)
		}
	}
}

func flattenLinkHealth(records map[string]*MeshLinkHealthRecord) []MeshLinkHealthRecord {
	if len(records) == 0 {
		return nil
	}
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]MeshLinkHealthRecord, 0, len(keys))
	for _, key := range keys {
		result = append(result, *records[key])
	}
	return result
}

func buildMeshAdapterSlots(paths meshFoundationPaths, activeProvider contracts.MeshProvider) []MeshAdapterSlot {
	return []MeshAdapterSlot{
		{
			Provider:   contracts.MeshProviderWireGuard,
			Status:     "active",
			Reserved:   false,
			Active:     activeProvider == contracts.MeshProviderWireGuard,
			ConfigRoot: paths.providerRoot,
			Summary:    "wireguard is the active overlay adapter for cross-node private traffic",
		},
		{
			Provider:   contracts.MeshProviderTailscale,
			Status:     "reserved",
			Reserved:   true,
			Active:     false,
			ConfigRoot: paths.tailscaleRoot,
			Summary:    "tailscale adapter slot is reserved for future provider integration",
		},
	}
}
