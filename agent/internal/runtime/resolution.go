package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"lazyops-agent/internal/contracts"
)

type runtimeDependencyResolver struct {
	runtimeCtx        RuntimeContext
	snapshot          *MeshFoundationSnapshot
	serviceCache      *ServiceMetadataCache
	routes            map[string]MeshRouteRecord
	peerIndex         map[string]MeshPeer
	routeFingerprint  string
	routeReportLoaded bool
}

type resolvedRoute struct {
	RouteScope            string
	ResolutionStatus      string
	ResolvedEndpoint      string
	ResolvedUpstream      string
	PlacementTargetID     string
	PlacementTargetKind   contracts.TargetKind
	PlacementPeerRef      string
	Provider              contracts.MeshProvider
	PublicFallbackBlocked bool
	InvalidationReasons   []string
	Reason                string
}

func newRuntimeDependencyResolver(runtimeRoot string, runtimeCtx RuntimeContext) (*runtimeDependencyResolver, error) {
	resolver := &runtimeDependencyResolver{
		runtimeCtx: runtimeCtx,
		routes:     make(map[string]MeshRouteRecord),
		peerIndex:  make(map[string]MeshPeer),
	}
	if runtimeCtx.Binding.RuntimeMode != contracts.RuntimeModeDistributedMesh {
		return resolver, nil
	}

	paths := (&MeshManager{runtimeRoot: runtimeRoot}).livePaths(runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID)
	if snapshot, err := loadMeshFoundationSnapshot(paths.liveStatePath); err == nil {
		resolver.snapshot = &snapshot
		for _, peer := range snapshot.Membership.Peers {
			resolver.peerIndex[peer.PeerRef] = peer
		}
		for _, route := range snapshot.RouteCache {
			resolver.routes[route.RouteKey] = route
		}
		resolver.routeFingerprint = meshRouteFingerprint(snapshot.RouteCache)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if cache, err := loadServiceMetadataCache(paths.liveServiceCachePath); err == nil {
		resolver.serviceCache = &cache
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if report, err := loadMeshRouteReport(paths.routeReportPath); err == nil {
		resolver.routeReportLoaded = true
		resolver.routes = make(map[string]MeshRouteRecord, len(report))
		for _, route := range report {
			resolver.routes[route.RouteKey] = route
		}
		resolver.routeFingerprint = meshRouteFingerprint(report)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return resolver, nil
}

func (r *runtimeDependencyResolver) PlacementFingerprint() string {
	return r.runtimeCtx.Runtime.PlacementFingerprint
}

func (r *runtimeDependencyResolver) RouteFingerprint() string {
	return r.routeFingerprint
}

func (r *runtimeDependencyResolver) InvalidationRules() []string {
	return append([]string(nil), resolutionInvalidationRules()...)
}

func (r *runtimeDependencyResolver) ResolveDependency(service ServiceRuntimeContext, dependency contracts.DependencyBindingPayload) DependencyResolutionView {
	targetService := r.lookupService(dependency.TargetService)
	sourcePlacement := resolvedPlacement(service, r.runtimeCtx.Binding, r.runtimeCtx.Runtime.PlacementByService)
	targetPlacement := resolvedPlacement(targetService, r.runtimeCtx.Binding, r.runtimeCtx.Runtime.PlacementByService)
	sourcePeerRef := meshPeerRef(sourcePlacement.TargetKind, sourcePlacement.TargetID)
	targetPeerRef := meshPeerRef(targetPlacement.TargetKind, targetPlacement.TargetID)

	resolution := resolvedRoute{
		RouteScope:          "local",
		ResolutionStatus:    "local_verified",
		ResolvedEndpoint:    derivedDependencyEndpoint(dependency),
		ResolvedUpstream:    derivedProxyUpstream(dependency),
		PlacementTargetID:   targetPlacement.TargetID,
		PlacementTargetKind: targetPlacement.TargetKind,
		PlacementPeerRef:    targetPeerRef,
		Reason:              "dependency target is placed on the same target as the source service",
	}

	if r.runtimeCtx.Binding.RuntimeMode == contracts.RuntimeModeDistributedMesh && sourcePeerRef != targetPeerRef {
		resolution = resolvedRoute{
			RouteScope:            "mesh_private",
			ResolutionStatus:      "planned",
			ResolvedEndpoint:      overlayDependencyEndpoint(dependency.Protocol, dependency.TargetService, targetPeerRef),
			ResolvedUpstream:      overlayDependencyUpstream(dependency.Protocol, dependency.TargetService, targetPeerRef),
			PlacementTargetID:     targetPlacement.TargetID,
			PlacementTargetKind:   targetPlacement.TargetKind,
			PlacementPeerRef:      targetPeerRef,
			Provider:              r.activeProvider(),
			PublicFallbackBlocked: true,
			Reason:                fmt.Sprintf("private overlay route to %s is waiting for route synchronization", targetPeerRef),
		}

		if route, ok := r.lookupRoute(service.Name, dependency); ok {
			resolution.RouteScope = firstNonEmpty(route.PathKind, resolution.RouteScope)
			resolution.ResolutionStatus = firstNonEmpty(route.Status, resolution.ResolutionStatus)
			resolution.Provider = firstNonEmptyProvider(route.Provider, resolution.Provider)
			resolution.PublicFallbackBlocked = route.PublicFallbackBlocked || resolution.PublicFallbackBlocked
			resolution.Reason = firstNonEmpty(route.Summary, resolution.Reason)
		} else if metadata, ok := r.lookupDependencyMetadata(service.Name, dependency); ok {
			resolution.RouteScope = firstNonEmpty(metadata.RouteScope, resolution.RouteScope)
			resolution.ResolutionStatus = firstNonEmpty(metadata.RouteStatus, resolution.ResolutionStatus)
			resolution.Provider = firstNonEmptyProvider(metadata.Provider, resolution.Provider)
			resolution.PublicFallbackBlocked = metadata.PublicFallbackBlocked || resolution.PublicFallbackBlocked
			resolution.Reason = resolutionReasonFromStatus(resolution.ResolutionStatus, targetPeerRef)
		}
	}

	resolution.InvalidationReasons = r.invalidationReasons(targetService.Name, targetPeerRef, resolution.ResolutionStatus, resolution.RouteScope == "mesh_private")

	return DependencyResolutionView{
		Alias:                 dependency.Alias,
		TargetService:         dependency.TargetService,
		Protocol:              dependency.Protocol,
		RouteScope:            resolution.RouteScope,
		ResolutionStatus:      resolution.ResolutionStatus,
		ResolvedEndpoint:      resolution.ResolvedEndpoint,
		ResolvedUpstream:      resolution.ResolvedUpstream,
		PlacementTargetID:     resolution.PlacementTargetID,
		PlacementTargetKind:   resolution.PlacementTargetKind,
		PlacementPeerRef:      resolution.PlacementPeerRef,
		Provider:              resolution.Provider,
		PublicFallbackBlocked: resolution.PublicFallbackBlocked,
		InvalidationReasons:   append([]string(nil), resolution.InvalidationReasons...),
		Reason:                resolution.Reason,
	}
}

func (r *runtimeDependencyResolver) ResolvePublicService(service ServiceRuntimeContext) GatewayRoute {
	placement := resolvedPlacement(service, r.runtimeCtx.Binding, r.runtimeCtx.Runtime.PlacementByService)
	placementPeerRef := meshPeerRef(placement.TargetKind, placement.TargetID)
	localPeerRef := meshPeerRef(r.runtimeCtx.Binding.TargetKind, firstNonEmpty(r.runtimeCtx.Binding.TargetID, r.runtimeCtx.Binding.TargetRef, r.runtimeCtx.Project.ProjectID))

	route := GatewayRoute{
		ServiceName:      service.Name,
		Port:             service.HealthCheck.Port,
		Upstream:         fmt.Sprintf("127.0.0.1:%d", service.HealthCheck.Port),
		RouteScope:       "local",
		ResolutionStatus: "local_verified",
		PlacementPeerRef: placementPeerRef,
		ResolutionReason: "public service is placed on the local target",
	}

	if r.runtimeCtx.Binding.RuntimeMode != contracts.RuntimeModeDistributedMesh || placementPeerRef == localPeerRef {
		return route
	}

	route.RouteScope = "mesh_private"
	route.Upstream = overlayGatewayUpstream(service.Name, placementPeerRef, service.HealthCheck.Port)
	route.Provider = r.activeProvider()
	route.PublicFallbackBlocked = true
	route.ResolutionStatus = "planned"
	route.ResolutionReason = fmt.Sprintf("public service is placed on remote peer %s and must use the private overlay", placementPeerRef)

	if peer, ok := r.peerIndex[placementPeerRef]; ok {
		switch peer.State {
		case MeshPeerStateActive:
			route.ResolutionStatus = "verified"
			route.ResolutionReason = fmt.Sprintf("remote public service is reachable through verified %s overlay", route.Provider)
		case MeshPeerStateDegraded:
			route.ResolutionStatus = "degraded"
			route.ResolutionReason = fmt.Sprintf("remote public service is on degraded peer %s", placementPeerRef)
		case MeshPeerStateLeaving, MeshPeerStateRemoved:
			route.ResolutionStatus = "blocked"
			route.ResolutionReason = fmt.Sprintf("remote public service is unavailable because peer %s is not present", placementPeerRef)
		default:
			route.ResolutionStatus = "planned"
			route.ResolutionReason = fmt.Sprintf("remote public service is waiting for peer %s to become active", placementPeerRef)
		}
	}
	route.InvalidationReasons = r.invalidationReasons(service.Name, placementPeerRef, route.ResolutionStatus, true)
	return route
}

func (r *runtimeDependencyResolver) lookupService(name string) ServiceRuntimeContext {
	if service, ok := r.runtimeCtx.Runtime.ServiceByName[name]; ok {
		return service
	}
	for _, service := range r.runtimeCtx.Services {
		if service.Name == name {
			return service
		}
	}
	return ServiceRuntimeContext{Name: name}
}

func (r *runtimeDependencyResolver) lookupRoute(serviceName string, dependency contracts.DependencyBindingPayload) (MeshRouteRecord, bool) {
	route, ok := r.routes[meshRouteKey(serviceName, dependency)]
	return route, ok
}

func (r *runtimeDependencyResolver) lookupDependencyMetadata(serviceName string, dependency contracts.DependencyBindingPayload) (ServiceDependencyMetadata, bool) {
	if r.serviceCache == nil {
		return ServiceDependencyMetadata{}, false
	}
	record, ok := r.serviceCache.Services[serviceName]
	if !ok {
		return ServiceDependencyMetadata{}, false
	}
	for _, item := range record.Dependencies {
		if item.Alias == dependency.Alias && item.TargetService == dependency.TargetService {
			return item, true
		}
	}
	return ServiceDependencyMetadata{}, false
}

func (r *runtimeDependencyResolver) invalidationReasons(targetServiceName, placementPeerRef, resolutionStatus string, remote bool) []string {
	reasons := make([]string, 0, 4)
	if !remote {
		return reasons
	}
	if r.serviceCache == nil {
		reasons = appendUniqueReason(reasons, "service_metadata_cache_missing")
	} else {
		if r.serviceCache.PlacementFingerprint != "" && r.serviceCache.PlacementFingerprint != r.runtimeCtx.Runtime.PlacementFingerprint {
			reasons = appendUniqueReason(reasons, "placement_assignments_changed")
		}
		if record, ok := r.serviceCache.Services[targetServiceName]; ok && record.PlacementPeerRef != "" && record.PlacementPeerRef != placementPeerRef {
			reasons = appendUniqueReason(reasons, "service_target_relocated")
		}
		if r.routeReportLoaded && r.serviceCache.RouteFingerprint != "" && r.routeFingerprint != "" && r.serviceCache.RouteFingerprint != r.routeFingerprint {
			reasons = appendUniqueReason(reasons, "overlay_route_status_changed")
		}
	}
	if !r.routeReportLoaded {
		reasons = appendUniqueReason(reasons, "overlay_route_sync_pending")
	}
	if resolutionStatus == "planned" || resolutionStatus == "degraded" || resolutionStatus == "blocked" {
		reasons = appendUniqueReason(reasons, "mesh_peer_health_changed")
	}
	if r.snapshot != nil && r.snapshot.Health.Status == "degraded" {
		reasons = appendUniqueReason(reasons, "mesh_health_degraded")
	}
	return reasons
}

func (r *runtimeDependencyResolver) activeProvider() contracts.MeshProvider {
	if r.snapshot != nil && r.snapshot.Provider != "" {
		return r.snapshot.Provider
	}
	if r.runtimeCtx.Binding.RuntimeMode == contracts.RuntimeModeDistributedMesh {
		return contracts.MeshProviderWireGuard
	}
	return ""
}

func resolutionInvalidationRules() []string {
	return []string{
		"placement_assignments_changed",
		"rollout_target_changed",
		"mesh_peer_health_changed",
		"overlay_route_status_changed",
	}
}

func placementFingerprint(binding contracts.DeploymentBindingPayload, services []ServiceRuntimeContext, placements map[string]contracts.PlacementAssignment) string {
	parts := []string{
		binding.BindingID,
		string(binding.RuntimeMode),
		string(binding.TargetKind),
		binding.TargetID,
		binding.TargetRef,
	}
	for _, service := range services {
		placement := resolvedPlacement(service, binding, placements)
		parts = append(parts, fmt.Sprintf("%s|%s|%s", service.Name, placement.TargetKind, placement.TargetID))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "||")))
	return "placement_" + hex.EncodeToString(sum[:8])
}

func meshRouteFingerprint(routes []MeshRouteRecord) string {
	if len(routes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(routes))
	for _, route := range routes {
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%s|%s", route.RouteKey, route.PathKind, route.Status, route.Provider, route.TargetPeerRef))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "||")))
	return "route_" + hex.EncodeToString(sum[:8])
}

func resolutionReasonFromStatus(status, peerRef string) string {
	switch status {
	case "verified":
		return fmt.Sprintf("private overlay route to %s is verified", peerRef)
	case "degraded":
		return fmt.Sprintf("private overlay route to %s is degraded", peerRef)
	case "blocked":
		return fmt.Sprintf("private overlay route to %s is blocked", peerRef)
	default:
		return fmt.Sprintf("private overlay route to %s is waiting for synchronization", peerRef)
	}
}

func overlayDependencyEndpoint(protocol, targetService, peerRef string) string {
	host := overlayServiceHost(targetService, peerRef)
	if protocol == "http" {
		return "http://" + host
	}
	return host
}

func overlayDependencyUpstream(protocol, targetService, peerRef string) string {
	host := overlayServiceHost(targetService, peerRef)
	if protocol == "http" {
		return "http://" + host
	}
	return host
}

func overlayGatewayUpstream(serviceName, peerRef string, port int) string {
	return fmt.Sprintf("%s:%d", overlayServiceHost(serviceName, peerRef), port)
}

func overlayServiceHost(serviceName, peerRef string) string {
	peerToken := sanitizeHostToken(strings.ReplaceAll(peerRef, ":", "-"))
	serviceToken := sanitizeHostToken(serviceName)
	return fmt.Sprintf("%s.%s.mesh.lazyops.internal", serviceToken, peerToken)
}

func appendUniqueReason(values []string, reason string) []string {
	for _, existing := range values {
		if existing == reason {
			return values
		}
	}
	return append(values, reason)
}

func firstNonEmptyProvider(values ...contracts.MeshProvider) contracts.MeshProvider {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func loadMeshRouteReport(path string) ([]MeshRouteRecord, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var routes []MeshRouteRecord
	if err := json.Unmarshal(payload, &routes); err != nil {
		return nil, err
	}
	return routes, nil
}
