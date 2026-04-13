package runtime

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func bindingNetworkName(projectID, bindingID string) string {
	name := fmt.Sprintf("lazyops-net-%s-%s",
		normalizeContainerToken(projectID),
		normalizeContainerToken(bindingID),
	)
	if len(name) > 63 {
		return name[:63]
	}
	return name
}

func serviceNetworkAlias(serviceName string) string {
	alias := strings.TrimSpace(serviceName)
	if alias == "" {
		return "app"
	}
	return normalizeContainerToken(alias)
}

func sidecarContainerName(projectID, bindingID, serviceName string) string {
	name := fmt.Sprintf("lazyops-sidecar-%s-%s-%s",
		normalizeContainerToken(projectID),
		normalizeContainerToken(bindingID),
		normalizeContainerToken(serviceName),
	)
	if len(name) > 63 {
		return name[:63]
	}
	return name
}

func localListenerPorts(service ServiceRuntimeContext) []int {
	ports := make([]int, 0, len(service.Dependencies))
	seen := make(map[int]struct{}, len(service.Dependencies))
	for _, dep := range service.Dependencies {
		port := dependencyLocalListenerPort(dep.LocalEndpoint)
		if port <= 0 {
			continue
		}
		if _, exists := seen[port]; exists {
			continue
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	return ports
}

func dependencyLocalListenerPort(localEndpoint string) int {
	value := strings.TrimSpace(localEndpoint)
	if value == "" {
		return 0
	}

	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err == nil && parsed != nil {
			if port, err := strconv.Atoi(parsed.Port()); err == nil {
				return port
			}
		}
	}

	host, port, err := net.SplitHostPort(value)
	if err == nil {
		if host == "" {
			host = "127.0.0.1"
		}
		if isLoopbackHost(host) {
			parsed, _ := strconv.Atoi(port)
			return parsed
		}
		return 0
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0
	}
	if !isLoopbackHost(strings.TrimSpace(parts[0])) {
		return 0
	}
	parsed, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return parsed
}

func serviceLocalListenerPortConflicts(service ServiceRuntimeContext, port int) bool {
	if port <= 0 {
		return false
	}
	for _, listenerPort := range localListenerPorts(service) {
		if listenerPort == port {
			return true
		}
	}
	return false
}

func standaloneDependencyTargetPort(targetService ServiceRuntimeContext, localEndpoint string) int {
	if isInternalServiceName(targetService.Name) {
		if port := dependencyLocalListenerPort(localEndpoint); port > 0 {
			return port
		}
		return declaredHealthcheckPort(targetService)
	}
	return effectiveRuntimePort(targetService)
}

func standaloneDependencyUpstream(runtimeCtx RuntimeContext, targetService ServiceRuntimeContext, dependencyProtocol string, fallbackPort int) string {
	host := serviceNetworkAlias(targetService.Name)
	port := fallbackPort
	if !isInternalServiceName(targetService.Name) {
		port = effectiveRuntimePort(targetService)
	}
	if port <= 0 {
		port = fallbackPort
	}
	if port <= 0 {
		return host
	}

	switch strings.ToLower(strings.TrimSpace(dependencyProtocol)) {
	case "http", "https":
		return fmt.Sprintf("http://%s:%d", host, port)
	default:
		return net.JoinHostPort(host, strconv.Itoa(port))
	}
}
