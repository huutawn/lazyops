package runtime

import (
	"strings"

	"lazyops-agent/internal/contracts"
)

const (
	standaloneRuntimePortRangeStart = 20000
	standaloneRuntimePortRangeEnd   = 45000
)

func effectiveRuntimePort(service ServiceRuntimeContext) int {
	if service.RuntimePort > 0 {
		return service.RuntimePort
	}
	if service.HealthCheck.Port > 0 {
		return service.HealthCheck.Port
	}
	return 0
}

func declaredHealthcheckPort(service ServiceRuntimeContext) int {
	return service.HealthCheck.Port
}

func isInternalServiceName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "lazyops-internal-service" || strings.HasPrefix(normalized, "lazyops-internal-")
}

func shouldAssignStandaloneRuntimePort(runtimeCtx RuntimeContext, service ServiceRuntimeContext) bool {
	if runtimeCtx.Binding.RuntimeMode != contracts.RuntimeModeStandalone {
		return false
	}
	if isInternalServiceName(service.Name) {
		return false
	}
	return true
}

func withRuntimeServices(runtimeCtx RuntimeContext, services []ServiceRuntimeContext) RuntimeContext {
	runtimeCtx.Services = append([]ServiceRuntimeContext(nil), services...)

	placementByService := make(map[string]contracts.PlacementAssignment, len(runtimeCtx.Revision.PlacementAssignments))
	for _, placement := range runtimeCtx.Revision.PlacementAssignments {
		placementByService[placement.ServiceName] = placement
	}

	serviceByName := make(map[string]ServiceRuntimeContext, len(runtimeCtx.Services))
	for _, service := range runtimeCtx.Services {
		serviceByName[service.Name] = service
	}

	runtimeCtx.Runtime = RuntimeDependencyContext{
		PlacementByService:   placementByService,
		ServiceByName:        serviceByName,
		PlacementFingerprint: placementFingerprint(runtimeCtx.Binding, runtimeCtx.Services, placementByService),
	}

	return runtimeCtx
}
