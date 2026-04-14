package runtime

import (
	"bytes"
	"log/slog"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type NodeMetricsConfig struct {
	CollectionInterval time.Duration
	FilesystemPath     string
}

func DefaultNodeMetricsConfig() NodeMetricsConfig {
	return NodeMetricsConfig{
		CollectionInterval: 10 * time.Second,
		FilesystemPath:     "/",
	}
}

type NodeMetricsSnapshot struct {
	CPUUsagePercent  float64   `json:"cpu_usage_percent"`
	MemoryUsedBytes  uint64    `json:"memory_used_bytes"`
	MemoryTotalBytes uint64    `json:"memory_total_bytes"`
	DiskUsedBytes    uint64    `json:"disk_used_bytes"`
	DiskTotalBytes   uint64    `json:"disk_total_bytes"`
	NetworkRXBytes   uint64    `json:"network_rx_bytes"`
	NetworkTXBytes   uint64    `json:"network_tx_bytes"`
	Goroutines       int       `json:"goroutines"`
	CollectedAt      time.Time `json:"collected_at"`
}

type cpuCounters struct {
	total uint64
	idle  uint64
}

type networkCounters struct {
	rx uint64
	tx uint64
}

type NodeMetricsCollector struct {
	logger *slog.Logger
	cfg    NodeMetricsConfig
	now    func() time.Time

	readProcStat func() ([]byte, error)
	readMemInfo  func() ([]byte, error)
	readNetDev   func() ([]byte, error)
	statfs       func(string, *syscall.Statfs_t) error

	mu          sync.Mutex
	last        *NodeMetricsSnapshot
	lastCPU     cpuCounters
	lastNetwork networkCounters
	hasCPU      bool
	hasNetwork  bool
	samples     int
}

func NewNodeMetricsCollector(logger *slog.Logger, cfg NodeMetricsConfig) *NodeMetricsCollector {
	defaults := DefaultNodeMetricsConfig()
	if cfg.CollectionInterval <= 0 {
		cfg.CollectionInterval = defaults.CollectionInterval
	}
	if strings.TrimSpace(cfg.FilesystemPath) == "" {
		cfg.FilesystemPath = defaults.FilesystemPath
	}

	return &NodeMetricsCollector{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		readProcStat: func() ([]byte, error) {
			return os.ReadFile("/proc/stat")
		},
		readMemInfo: func() ([]byte, error) {
			return os.ReadFile("/proc/meminfo")
		},
		readNetDev: func() ([]byte, error) {
			return os.ReadFile("/proc/net/dev")
		},
		statfs: syscall.Statfs,
	}
}

func (c *NodeMetricsCollector) Collect() NodeMetricsSnapshot {
	snapshot := NodeMetricsSnapshot{
		CollectedAt: c.now(),
		Goroutines:  runtime.NumGoroutine(),
	}

	totalMemory, usedMemory, err := c.readMemoryUsage()
	if err != nil {
		c.logReadWarning("memory", err)
	} else {
		snapshot.MemoryTotalBytes = totalMemory
		snapshot.MemoryUsedBytes = usedMemory
	}

	diskUsed, diskTotal, err := c.readDiskUsage()
	if err != nil {
		c.logReadWarning("disk", err)
	} else {
		snapshot.DiskUsedBytes = diskUsed
		snapshot.DiskTotalBytes = diskTotal
	}

	network, networkErr := c.readNetworkCounters()
	if networkErr != nil {
		c.logReadWarning("network", networkErr)
	}

	cpu, cpuErr := c.readCPUCounters()
	if cpuErr != nil {
		c.logReadWarning("cpu", cpuErr)
	}

	c.mu.Lock()
	if cpuErr == nil {
		snapshot.CPUUsagePercent = c.computeCPUUsageLocked(cpu)
	}
	if networkErr == nil {
		snapshot.NetworkRXBytes, snapshot.NetworkTXBytes = c.computeNetworkDeltaLocked(network)
	}
	c.last = &snapshot
	c.samples++
	c.mu.Unlock()

	return snapshot
}

func (c *NodeMetricsCollector) FeedToAggregator(agg *MetricAggregator, snapshot NodeMetricsSnapshot) {
	if agg == nil {
		return
	}

	if snapshot.CPUUsagePercent >= 0 {
		agg.Record("cpu", snapshot.CPUUsagePercent)
	}
	if snapshot.MemoryUsedBytes > 0 {
		agg.Record("ram", float64(snapshot.MemoryUsedBytes))
	}
	if snapshot.DiskUsedBytes > 0 {
		agg.Record("disk", float64(snapshot.DiskUsedBytes))
	}
	if snapshot.NetworkRXBytes > 0 {
		agg.Record("network_in", float64(snapshot.NetworkRXBytes))
	}
	if snapshot.NetworkTXBytes > 0 {
		agg.Record("network_out", float64(snapshot.NetworkTXBytes))
	}
}

func (c *NodeMetricsCollector) LastSnapshot() *NodeMetricsSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

func (c *NodeMetricsCollector) Stats() (samples int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.samples
}

func (c *NodeMetricsCollector) readCPUCounters() (cpuCounters, error) {
	if c.readProcStat == nil {
		return cpuCounters{}, os.ErrNotExist
	}

	raw, err := c.readProcStat()
	if err != nil {
		return cpuCounters{}, err
	}
	return parseProcStat(raw)
}

func (c *NodeMetricsCollector) readMemoryUsage() (total, used uint64, err error) {
	if c.readMemInfo == nil {
		return 0, 0, os.ErrNotExist
	}

	raw, err := c.readMemInfo()
	if err != nil {
		return 0, 0, err
	}
	return parseMemInfo(raw)
}

func (c *NodeMetricsCollector) readDiskUsage() (used, total uint64, err error) {
	if c.statfs == nil {
		return 0, 0, os.ErrNotExist
	}

	var fs syscall.Statfs_t
	if err := c.statfs(c.cfg.FilesystemPath, &fs); err != nil {
		return 0, 0, err
	}

	total = fs.Blocks * uint64(fs.Bsize)
	free := fs.Bavail * uint64(fs.Bsize)
	if total < free {
		return 0, total, nil
	}
	used = total - free
	return used, total, nil
}

func (c *NodeMetricsCollector) readNetworkCounters() (networkCounters, error) {
	if c.readNetDev == nil {
		return networkCounters{}, os.ErrNotExist
	}

	raw, err := c.readNetDev()
	if err != nil {
		return networkCounters{}, err
	}
	return parseNetDev(raw)
}

func (c *NodeMetricsCollector) computeCPUUsageLocked(current cpuCounters) float64 {
	if !c.hasCPU {
		c.lastCPU = current
		c.hasCPU = true
		return 0
	}

	if current.total < c.lastCPU.total || current.idle < c.lastCPU.idle {
		c.lastCPU = current
		return 0
	}
	totalDelta := current.total - c.lastCPU.total
	idleDelta := current.idle - c.lastCPU.idle
	c.lastCPU = current
	if totalDelta == 0 {
		return 0
	}

	usage := 100 * (1 - float64(idleDelta)/float64(totalDelta))
	if math.IsNaN(usage) || math.IsInf(usage, 0) {
		return 0
	}
	return clampFloat(usage, 0, 100)
}

func (c *NodeMetricsCollector) computeNetworkDeltaLocked(current networkCounters) (uint64, uint64) {
	if !c.hasNetwork {
		c.lastNetwork = current
		c.hasNetwork = true
		return 0, 0
	}

	var rx uint64
	if current.rx >= c.lastNetwork.rx {
		rx = current.rx - c.lastNetwork.rx
	}
	var tx uint64
	if current.tx >= c.lastNetwork.tx {
		tx = current.tx - c.lastNetwork.tx
	}
	c.lastNetwork = current
	return rx, tx
}

func (c *NodeMetricsCollector) logReadWarning(kind string, err error) {
	if c == nil || c.logger == nil || err == nil {
		return
	}
	c.logger.Warn("failed to read node metric",
		"metric", kind,
		"error", err,
	)
}

func parseProcStat(raw []byte) (cpuCounters, error) {
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		fields := strings.Fields(string(line))
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}

		var total uint64
		values := make([]uint64, 0, len(fields)-1)
		for _, field := range fields[1:] {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return cpuCounters{}, err
			}
			total += value
			values = append(values, value)
		}

		idle := uint64(0)
		if len(values) > 3 {
			idle += values[3]
		}
		if len(values) > 4 {
			idle += values[4]
		}

		return cpuCounters{total: total, idle: idle}, nil
	}

	return cpuCounters{}, os.ErrNotExist
}

func parseMemInfo(raw []byte) (total, used uint64, err error) {
	var memTotalKB uint64
	var memAvailableKB uint64
	var memFreeKB uint64
	var buffersKB uint64
	var cachedKB uint64

	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		fields := strings.Fields(string(line))
		if len(fields) < 2 {
			continue
		}

		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}

		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			memTotalKB = value
		case "MemAvailable":
			memAvailableKB = value
		case "MemFree":
			memFreeKB = value
		case "Buffers":
			buffersKB = value
		case "Cached":
			cachedKB = value
		}
	}

	if memTotalKB == 0 {
		return 0, 0, os.ErrNotExist
	}
	if memAvailableKB == 0 {
		memAvailableKB = memFreeKB + buffersKB + cachedKB
	}

	total = memTotalKB * 1024
	available := memAvailableKB * 1024
	if total < available {
		return total, 0, nil
	}
	used = total - available
	return total, used, nil
}

func parseNetDev(raw []byte) (networkCounters, error) {
	var totals networkCounters
	found := false

	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		text := strings.TrimSpace(string(line))
		if text == "" || !strings.Contains(text, ":") {
			continue
		}

		parts := strings.SplitN(text, ":", 2)
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		if !includeNetworkInterface(iface) {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		rx, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return networkCounters{}, err
		}
		tx, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return networkCounters{}, err
		}

		totals.rx += rx
		totals.tx += tx
		found = true
	}

	if !found {
		return networkCounters{}, os.ErrNotExist
	}
	return totals, nil
}

func includeNetworkInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case name == "", name == "lo":
		return false
	case strings.HasPrefix(name, "docker"),
		strings.HasPrefix(name, "veth"),
		strings.HasPrefix(name, "br-"),
		strings.HasPrefix(name, "cni"),
		strings.HasPrefix(name, "flannel"),
		strings.HasPrefix(name, "virbr"),
		strings.HasPrefix(name, "tunl"),
		strings.HasPrefix(name, "ifb"),
		strings.HasPrefix(name, "dummy"),
		strings.HasPrefix(name, "wg"),
		strings.HasPrefix(name, "tailscale"),
		strings.HasPrefix(name, "zt"),
		strings.HasPrefix(name, "kube"),
		strings.HasPrefix(name, "tap"),
		strings.HasPrefix(name, "vmnet"):
		return false
	default:
		return true
	}
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
