package runtime

import (
	"context"
	"fmt"
	"sync"
)

const (
	RuntimeModeStandalone      = "standalone"
	RuntimeModeDistributedMesh = "distributed-mesh"
	RuntimeModeDistributedK3s  = "distributed-k3s"
)

var validRuntimeModes = map[string]struct{}{
	RuntimeModeStandalone:      {},
	RuntimeModeDistributedMesh: {},
	RuntimeModeDistributedK3s:  {},
}

type DriverCapability string

const (
	CapabilityWorkloadScheduling DriverCapability = "workload_scheduling"
	CapabilityMeshRouting        DriverCapability = "mesh_routing"
	CapabilityKubernetesNative   DriverCapability = "kubernetes_native"
	CapabilityLocalProcess       DriverCapability = "local_process"
	CapabilityGatewayManagement  DriverCapability = "gateway_management"
	CapabilityHealthProbing      DriverCapability = "health_probing"
	CapabilityScaleToZero        DriverCapability = "scale_to_zero"
)

type DriverInfo struct {
	Mode         string
	Capabilities []DriverCapability
}

type RuntimeDriver interface {
	Mode() string
	Info() DriverInfo
	ValidateTarget(ctx context.Context, target TargetSpec) error
	PlanRollout(ctx context.Context, req RolloutRequest) (*RolloutPlan, error)
	ExecuteCommand(ctx context.Context, cmd AgentCommand) (*CommandResult, error)
}

type TargetSpec struct {
	TargetKind  string
	TargetID    string
	RuntimeMode string
	Metadata    map[string]any
}

type RolloutRequest struct {
	ProjectID       string
	RevisionID      string
	BindingID       string
	RevisionPayload map[string]any
}

type RolloutPlan struct {
	Steps       []RolloutStep
	RuntimeMode string
	TargetKind  string
}

type RolloutStep struct {
	Kind    string
	Command AgentCommand
}

type AgentCommand struct {
	Type          string         `json:"type"`
	RequestID     string         `json:"request_id"`
	CorrelationID string         `json:"correlation_id"`
	AgentID       string         `json:"agent_id"`
	ProjectID     string         `json:"project_id"`
	Source        string         `json:"source"`
	OccurredAt    string         `json:"occurred_at"`
	Payload       map[string]any `json:"payload"`
}

type CommandResult struct {
	RequestID string
	Status    string
	Output    map[string]any
	Error     string
}

type Registry struct {
	mu      sync.RWMutex
	drivers map[string]RuntimeDriver
}

func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[string]RuntimeDriver),
	}
}

func (r *Registry) Register(driver RuntimeDriver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mode := driver.Mode()
	if _, ok := r.drivers[mode]; ok {
		return fmt.Errorf("runtime driver already registered for mode %q", mode)
	}
	r.drivers[mode] = driver
	return nil
}

func (r *Registry) Get(mode string) (RuntimeDriver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, ok := r.drivers[mode]
	if !ok {
		return nil, fmt.Errorf("no runtime driver registered for mode %q", mode)
	}
	return driver, nil
}

func (r *Registry) List() []DriverInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]DriverInfo, 0, len(r.drivers))
	for _, driver := range r.drivers {
		out = append(out, driver.Info())
	}
	return out
}

func IsValidRuntimeMode(mode string) bool {
	_, ok := validRuntimeModes[mode]
	return ok
}

func ValidateRuntimeMode(mode string) error {
	if !IsValidRuntimeMode(mode) {
		return fmt.Errorf("invalid runtime mode %q, must be one of: %s, %s, %s",
			mode, RuntimeModeStandalone, RuntimeModeDistributedMesh, RuntimeModeDistributedK3s)
	}
	return nil
}
