package runtime

import (
	"context"
	"fmt"
)

type DistributedK3sDriver struct{}

func NewDistributedK3sDriver() *DistributedK3sDriver {
	return &DistributedK3sDriver{}
}

func (d *DistributedK3sDriver) Mode() string {
	return RuntimeModeDistributedK3s
}

func (d *DistributedK3sDriver) Info() DriverInfo {
	return DriverInfo{
		Mode: RuntimeModeDistributedK3s,
		Capabilities: []DriverCapability{
			CapabilityKubernetesNative,
			CapabilityHealthProbing,
			CapabilityGatewayManagement,
		},
	}
}

func (d *DistributedK3sDriver) ValidateTarget(ctx context.Context, target TargetSpec) error {
	if target.TargetKind != "cluster" {
		return fmt.Errorf("distributed-k3s driver requires target kind %q, got %q", "cluster", target.TargetKind)
	}
	if target.TargetID == "" {
		return fmt.Errorf("distributed-k3s driver requires non-empty target_id")
	}
	return nil
}

func (d *DistributedK3sDriver) PlanRollout(ctx context.Context, req RolloutRequest) (*RolloutPlan, error) {
	return &RolloutPlan{
		Steps: []RolloutStep{
			{
				Kind: "render_gateway_config",
				Command: AgentCommand{
					Type:      "render_gateway_config",
					ProjectID: req.ProjectID,
					Source:    "distributed_k3s_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "reconcile",
				Command: AgentCommand{
					Type:      "reconcile_revision",
					ProjectID: req.ProjectID,
					Source:    "distributed_k3s_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "health_gate",
				Command: AgentCommand{
					Type:      "run_health_gate",
					ProjectID: req.ProjectID,
					Source:    "distributed_k3s_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "promote",
				Command: AgentCommand{
					Type:      "promote_release",
					ProjectID: req.ProjectID,
					Source:    "distributed_k3s_driver",
					Payload:   req.RevisionPayload,
				},
			},
		},
		RuntimeMode: RuntimeModeDistributedK3s,
		TargetKind:  "cluster",
	}, nil
}

func (d *DistributedK3sDriver) ExecuteCommand(ctx context.Context, cmd AgentCommand) (*CommandResult, error) {
	if cmd.Type == "" {
		return nil, fmt.Errorf("command type is required")
	}
	if err := d.guardK3sBoundary(cmd); err != nil {
		return nil, err
	}

	validTypes := map[string]bool{
		"render_gateway_config":   true,
		"reconcile_revision":      true,
		"start_release_candidate": true,
		"run_health_gate":         true,
		"promote_release":         true,
		"rollback_release":        true,
		"garbage_collect_runtime": true,
	}

	if !validTypes[cmd.Type] {
		return nil, fmt.Errorf("unsupported command type %q for k3s driver", cmd.Type)
	}

	return &CommandResult{
		RequestID: cmd.RequestID,
		Status:    "dispatched",
		Output: map[string]any{
			"driver":  "distributed_k3s",
			"command": cmd.Type,
		},
	}, nil
}

func (d *DistributedK3sDriver) guardK3sBoundary(cmd AgentCommand) error {
	forbiddenCommands := map[string]struct{}{
		"docker_run":    {},
		"docker_stop":   {},
		"docker_rm":     {},
		"direct_deploy": {},
		"sleep_service": {},
		"wake_service":  {},
		"scale_to_zero": {},
	}
	if _, ok := forbiddenCommands[cmd.Type]; ok {
		return fmt.Errorf("command %q is forbidden in distributed-k3s mode: workload scheduling must go through Kubernetes", cmd.Type)
	}
	return nil
}
