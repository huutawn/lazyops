package runtime

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"lazyops-agent/internal/contracts"
)

func automaticServiceDiscoveryEnvMap(projectID string, runtimeMode contracts.RuntimeMode, services []ServiceRuntimeContext) map[string]string {
	if len(services) == 0 {
		return map[string]string{}
	}

	env := make(map[string]string, len(services)*6)
	for _, service := range services {
		appendAutomaticServiceDiscoveryEnv(env, projectID, runtimeMode, service)
	}
	return env
}

func appendAutomaticServiceDiscoveryEnv(env map[string]string, projectID string, runtimeMode contracts.RuntimeMode, service ServiceRuntimeContext) {
	alias := sanitizeEnvKey(service.Name)
	if alias == "" {
		return
	}

	host := automaticServiceDiscoveryHost(projectID, runtimeMode, service)
	port := effectiveRuntimePort(service)
	if strings.TrimSpace(host) == "" || port <= 0 {
		return
	}

	url := automaticServiceDiscoveryURL(service, host, port)
	env[alias+"_HOST"] = host
	env[alias+"_PORT"] = strconv.Itoa(port)
	env[alias+"_URL"] = url
	env["LAZYOPS_SERVICE_"+alias+"_HOST"] = host
	env["LAZYOPS_SERVICE_"+alias+"_PORT"] = strconv.Itoa(port)
	env["LAZYOPS_SERVICE_"+alias+"_URL"] = url
}

func automaticServiceDiscoveryHost(projectID string, runtimeMode contracts.RuntimeMode, service ServiceRuntimeContext) string {
	if runtimeMode == contracts.RuntimeModeStandalone {
		return serviceNetworkAlias(service.Name)
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return strings.TrimSpace(service.Name)
	}
	return fmt.Sprintf("%s.%s.%s", strings.TrimSpace(service.Name), projectID, "lazyops.internal")
}

func automaticServiceDiscoveryURL(service ServiceRuntimeContext, host string, port int) string {
	scheme := automaticServiceDiscoveryScheme(service)
	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func automaticServiceDiscoveryScheme(service ServiceRuntimeContext) string {
	switch strings.ToLower(strings.TrimSpace(service.HealthCheck.Protocol)) {
	case "https", "grpc":
		return "https"
	case "tcp":
		return "tcp"
	case "http":
		return "http"
	}

	switch strings.ToLower(strings.TrimSpace(service.Kind)) {
	case "postgres", "mysql", "mongodb", "redis", "rabbitmq", "kafka":
		return "tcp"
	default:
		return "http"
	}
}
