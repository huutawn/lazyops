package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

// DockerPortDiscovery discovers service ports by inspecting running Docker containers.
// It complements ServicePortDiscovery (which uses ss/netstat) by providing container
// metadata like labels, names, and explicit port mappings.
type DockerPortDiscovery struct {
	logger *slog.Logger
}

func NewDockerPortDiscovery(logger *slog.Logger) *DockerPortDiscovery {
	return &DockerPortDiscovery{logger: logger}
}

// ContainerPortInfo holds port information from a Docker container
type ContainerPortInfo struct {
	ContainerID   string            `json:"container_id"`
	ContainerName string            `json:"container_name"`
	Image         string            `json:"image"`
	Ports         []ExposedPort     `json:"ports"`
	Labels        map[string]string `json:"labels"`
	Networks      []string          `json:"networks"`
}

// ExposedPort holds information about an exposed container port
type ExposedPort struct {
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"` // "tcp" or "udp"
	IP            string `json:"ip"`       // binding IP (e.g., "127.0.0.1", "0.0.0.0")
}

// DiscoverContainerPorts inspects all running Docker containers and returns their port mappings
func (d *DockerPortDiscovery) DiscoverContainerPorts(ctx context.Context) ([]ContainerPortInfo, error) {
	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		d.logger.Debug("docker not found in PATH, skipping container port discovery")
		return nil, nil
	}

	// List running container IDs
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list docker containers: %w", err)
	}

	containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(containerIDs) == 0 || (len(containerIDs) == 1 && containerIDs[0] == "") {
		return nil, nil
	}

	var results []ContainerPortInfo
	for _, cid := range containerIDs {
		cid = strings.TrimSpace(cid)
		if cid == "" {
			continue
		}

		info, err := d.inspectContainer(ctx, cid)
		if err != nil {
			d.logger.Warn("failed to inspect docker container",
				"container_id", cid,
				"error", err,
			)
			continue
		}
		results = append(results, info)
	}

	return results, nil
}

// DiscoverServicePorts discovers ports from docker containers and returns
// them in the same format as ServicePortDiscovery for compatibility
func (d *DockerPortDiscovery) DiscoverServicePorts(ctx context.Context) ([]ServicePortInfo, error) {
	containers, err := d.DiscoverContainerPorts(ctx)
	if err != nil {
		return nil, err
	}

	var services []ServicePortInfo
	for _, container := range containers {
		for _, port := range container.Ports {
			// Use host port for external access, container port for internal
			host := port.IP
			if host == "0.0.0.0" || host == "::" {
				host = "127.0.0.1"
			}

			services = append(services, ServicePortInfo{
				Port:     port.HostPort,
				Host:     host,
				Protocol: port.Protocol,
				Process:  container.ContainerName,
				PID:      0, // Docker container, no direct PID
			})
		}
	}

	return services, nil
}

// FindServiceByPort finds the container that exposes a given port
func (d *DockerPortDiscovery) FindServiceByPort(ctx context.Context, port int) (*ContainerPortInfo, *ExposedPort, error) {
	containers, err := d.DiscoverContainerPorts(ctx)
	if err != nil {
		return nil, nil, err
	}

	for i := range containers {
		for j := range containers[i].Ports {
			if containers[i].Ports[j].HostPort == port || containers[i].Ports[j].ContainerPort == port {
				return &containers[i], &containers[i].Ports[j], nil
			}
		}
	}

	return nil, nil, nil
}

func (d *DockerPortDiscovery) inspectContainer(ctx context.Context, containerID string) (ContainerPortInfo, error) {
	info := ContainerPortInfo{
		ContainerID: containerID,
		Labels:      make(map[string]string),
	}

	// Get container details with inspect
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format",
		`{{.Name}}|{{.Config.Image}}|{{range $p, $conf := .NetworkSettings.Ports}}{{$p}}={{range $conf}}{{.HostIp}}:{{.HostPort}};{{end}}|{{end}}`,
		containerID,
	)
	output, err := cmd.Output()
	if err != nil {
		return info, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 3)
	if len(parts) < 3 {
		return info, fmt.Errorf("unexpected docker inspect output format for %s", containerID)
	}

	info.ContainerName = strings.TrimPrefix(parts[0], "/")
	info.Image = parts[1]

	// Parse ports (format: "8080/tcp=0.0.0.0:8080;;")
	portSection := parts[2]
	info.Ports = parseDockerPorts(portSection)

	// Get labels
	labelsCmd := exec.CommandContext(ctx, "docker", "inspect", "--format",
		`{{range $k, $v := .Config.Labels}}{{$k}}={{$v}}
{{end}}`,
		containerID,
	)
	labelsOutput, err := labelsCmd.Output()
	if err == nil {
		info.Labels = parseDockerLabels(string(labelsOutput))
	}

	// Get networks
	networksCmd := exec.CommandContext(ctx, "docker", "inspect", "--format",
		`{{range $k, $v := .NetworkSettings.Networks}}{{$k}}
{{end}}`,
		containerID,
	)
	networksOutput, err := networksCmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(networksOutput), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				info.Networks = append(info.Networks, line)
			}
		}
	}

	return info, nil
}

// parseDockerPorts parses docker port mapping output
func parseDockerPorts(portSection string) []ExposedPort {
	var ports []ExposedPort
	// Format: "8080/tcp=0.0.0.0:8080;;" or "5432/tcp=127.0.0.1:5432;;"
	entries := strings.Split(portSection, "|")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Split container_port/protocol=host_bindings
		eqIdx := strings.Index(entry, "=")
		if eqIdx == -1 {
			continue
		}

		containerPart := entry[:eqIdx]
		hostBindings := entry[eqIdx+1:]

		// Parse container port and protocol
		slashIdx := strings.LastIndex(containerPart, "/")
		if slashIdx == -1 {
			continue
		}

		containerPort, err := strconv.Atoi(containerPart[:slashIdx])
		if err != nil {
			continue
		}
		protocol := containerPart[slashIdx+1:]

		// Parse host bindings
		bindings := strings.Split(hostBindings, ";")
		for _, binding := range bindings {
			binding = strings.TrimSpace(binding)
			if binding == "" {
				continue
			}

			colonIdx := strings.LastIndex(binding, ":")
			if colonIdx == -1 {
				continue
			}

			hostIP := binding[:colonIdx]
			hostPort, err := strconv.Atoi(binding[colonIdx+1:])
			if err != nil {
				continue
			}

			ports = append(ports, ExposedPort{
				ContainerPort: containerPort,
				HostPort:      hostPort,
				Protocol:      protocol,
				IP:            hostIP,
			})
		}
	}

	return ports
}

// parseDockerLabels parses docker labels output
func parseDockerLabels(output string) map[string]string {
	labels := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}
		labels[line[:eqIdx]] = line[eqIdx+1:]
	}
	return labels
}

// GetContainerJSON returns the full docker inspect JSON for a container
func (d *DockerPortDiscovery) GetContainerJSON(ctx context.Context, containerID string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse docker inspect output: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no inspect data for container %s", containerID)
	}

	return result[0], nil
}
