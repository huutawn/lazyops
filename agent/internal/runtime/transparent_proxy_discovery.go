package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

// ServicePortDiscovery discovers listening ports and their processes
// for transparent proxy auto-configuration.
type ServicePortDiscovery struct {
	logger *slog.Logger
}

func NewServicePortDiscovery(logger *slog.Logger) *ServicePortDiscovery {
	return &ServicePortDiscovery{logger: logger}
}

// ServicePortInfo holds information about a listening port
type ServicePortInfo struct {
	Port     int    `json:"port"`
	Host     string `json:"host"`
	Protocol string `json:"protocol"`
	Process  string `json:"process"`
	PID      int    `json:"pid"`
}

// DiscoverServicePorts discovers all listening ports and their processes
// using the 'ss' command (socket statistics).
func (d *ServicePortDiscovery) DiscoverServicePorts(ctx context.Context) ([]ServicePortInfo, error) {
	// Use 'ss' to discover listening ports
	cmd := exec.CommandContext(ctx, "ss", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		// Try netstat as fallback
		cmd = exec.CommandContext(ctx, "netstat", "-tlnp")
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to discover ports (ss and netstat both failed): %w", err)
		}
		return parseNetstatOutput(string(output))
	}

	return parseSSOutput(string(output))
}

func parseSSOutput(output string) ([]ServicePortInfo, error) {
	var services []ServicePortInfo
	lines := strings.Split(output, "\n")

	// Skip header line
	for _, line := range lines[1:] {
		parts := strings.Fields(line)
		if len(parts) < 6 {
			continue
		}

		// Parse local address (e.g., "0.0.0.0:3000" or "127.0.0.1:8000" or "*:3000")
		localAddr := parts[4]
		host, portStr, err := parseAddress(localAddr)
		if err != nil {
			continue
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		// Parse process info (e.g., "users:(("node",pid=1234,fd=10))")
		procInfo := ""
		if len(parts) > 6 {
			procInfo = parts[6]
		}
		procName, pid := parseProcessInfo(procInfo)

		services = append(services, ServicePortInfo{
			Port:     port,
			Host:     host,
			Protocol: "tcp",
			Process:  procName,
			PID:      pid,
		})
	}

	return services, nil
}

func parseNetstatOutput(output string) ([]ServicePortInfo, error) {
	var services []ServicePortInfo
	lines := strings.Split(output, "\n")

	// Skip header lines
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		// Check if this is a listening socket
		isListening := false
		for _, part := range parts {
			if part == "LISTEN" {
				isListening = true
				break
			}
		}
		if !isListening {
			continue
		}

		// Parse local address (e.g., "0.0.0.0:3000" or "127.0.0.1:8000")
		localAddr := parts[3]
		host, portStr, err := parseAddress(localAddr)
		if err != nil {
			continue
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		// Parse process info (last column, e.g., "1234/node")
		procInfo := ""
		if len(parts) > 6 {
			procInfo = parts[len(parts)-1]
		}
		procName, pid := parseNetstatProcessInfo(procInfo)

		services = append(services, ServicePortInfo{
			Port:     port,
			Host:     host,
			Protocol: "tcp",
			Process:  procName,
			PID:      pid,
		})
	}

	return services, nil
}

func parseAddress(addr string) (host, port string, err error) {
	// Handle wildcards like "*:3000" or "[::]:3000"
	addr = strings.TrimPrefix(addr, "*")

	// Handle empty host (e.g., ":3000")
	if addr != "" && addr[0] == ':' {
		addr = "0.0.0.0" + addr
	}

	// Handle IPv6
	addr = strings.TrimPrefix(addr, "[::]")
	if strings.HasPrefix(addr, "::") {
		addr = "0.0.0.0" + addr[2:]
	}

	lastColon := strings.LastIndex(addr, ":")
	if lastColon == -1 {
		return "", "", fmt.Errorf("invalid address: %s", addr)
	}
	return addr[:lastColon], addr[lastColon+1:], nil
}

func parseProcessInfo(info string) (name string, pid int) {
	// Parse "users:(("node",pid=1234,fd=10))"
	start := strings.Index(info, "(\"")
	if start == -1 {
		return "", 0
	}

	end := strings.Index(info[start+2:], "\"")
	if end == -1 {
		return "", 0
	}

	name = info[start+2 : start+2+end]

	// Extract PID
	pidIndex := strings.Index(info, "pid=")
	if pidIndex == -1 {
		return name, 0
	}

	pidStr := info[pidIndex+4:]
	pidEnd := strings.IndexAny(pidStr, ",)")
	if pidEnd == -1 {
		return name, 0
	}

	pid, _ = strconv.Atoi(pidStr[:pidEnd])
	return name, pid
}

func parseNetstatProcessInfo(info string) (name string, pid int) {
	// Parse "1234/node" or "1234/docker-proxy"
	if info == "" || info == "-" {
		return "", 0
	}

	parts := strings.SplitN(info, "/", 2)
	if len(parts) != 2 {
		return info, 0
	}

	pid, _ = strconv.Atoi(parts[0])
	name = parts[1]
	return name, pid
}
