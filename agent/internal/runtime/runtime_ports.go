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
	if service.TargetPort > 0 {
		return service.TargetPort
	}
	if service.ServicePort > 0 {
		return service.ServicePort
	}
	for _, port := range service.DetectedPorts {
		if port.Port > 0 {
			return port.Port
		}
	}
	switch strings.ToLower(strings.TrimSpace(service.Kind)) {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "redis":
		return 6379
	case "rabbitmq":
		return 5672
	default:
		return 8080
	}
}

func declaredHealthcheckPort(service ServiceRuntimeContext) int {
	if service.HealthCheck.Port > 0 {
		return service.HealthCheck.Port
	}
	if service.TargetPort > 0 {
		return service.TargetPort
	}
	if service.ServicePort > 0 {
		return service.ServicePort
	}
	for _, port := range service.DetectedPorts {
		if port.Port > 0 {
			return port.Port
		}
	}
	return 0
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
