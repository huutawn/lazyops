package runtime

import (
	"context"
	"fmt"
)

type DistributedMeshDriver struct{}

func NewDistributedMeshDriver() *DistributedMeshDriver {
	return &DistributedMeshDriver{}
}

func (d *DistributedMeshDriver) Mode() string {
	return RuntimeModeDistributedMesh
}

func (d *DistributedMeshDriver) Info() DriverInfo {
	return DriverInfo{
		Mode: RuntimeModeDistributedMesh,
		Capabilities: []DriverCapability{
			CapabilityMeshRouting,
			CapabilityHealthProbing,
			CapabilityScaleToZero,
			CapabilityGatewayManagement,
		},
	}
}

func (d *DistributedMeshDriver) ValidateTarget(ctx context.Context, target TargetSpec) error {
	if target.TargetKind != "mesh_network" {
		return fmt.Errorf("distributed-mesh driver requires target kind %q, got %q", "mesh_network", target.TargetKind)
	}
	if target.TargetID == "" {
		return fmt.Errorf("distributed-mesh driver requires non-empty target_id")
	}
	return nil
}

func (d *DistributedMeshDriver) PlanRollout(ctx context.Context, req RolloutRequest) (*RolloutPlan, error) {
	return &RolloutPlan{
		Steps: []RolloutStep{
			{
				Kind: "ensure_mesh_peers",
				Command: AgentCommand{
					Type:      "ensure_mesh_peer",
					ProjectID: req.ProjectID,
					Source:    "distributed_mesh_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "sync_overlay_routes",
				Command: AgentCommand{
					Type:      "sync_overlay_routes",
					ProjectID: req.ProjectID,
					Source:    "distributed_mesh_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "render_sidecars",
				Command: AgentCommand{
					Type:      "render_sidecars",
					ProjectID: req.ProjectID,
					Source:    "distributed_mesh_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "render_gateway_config",
				Command: AgentCommand{
					Type:      "render_gateway_config",
					ProjectID: req.ProjectID,
					Source:    "distributed_mesh_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "reconcile",
				Command: AgentCommand{
					Type:      "reconcile_revision",
					ProjectID: req.ProjectID,
					Source:    "distributed_mesh_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "health_gate",
				Command: AgentCommand{
					Type:      "run_health_gate",
					ProjectID: req.ProjectID,
					Source:    "distributed_mesh_driver",
					Payload:   map[string]any{"revision_id": req.RevisionID},
				},
			},
			{
				Kind: "promote",
				Command: AgentCommand{
					Type:      "promote_release",
					ProjectID: req.ProjectID,
					Source:    "distributed_mesh_driver",
					Payload:   map[string]any{"revision_id": req.RevisionID},
				},
			},
		},
		RuntimeMode: RuntimeModeDistributedMesh,
		TargetKind:  "mesh_network",
	}, nil
}

func (d *DistributedMeshDriver) ExecuteCommand(ctx context.Context, cmd AgentCommand) (*CommandResult, error) {
	if cmd.Type == "" {
		return nil, fmt.Errorf("command type is required")
	}

	validTypes := map[string]bool{
		"ensure_mesh_peer":        true,
		"sync_overlay_routes":     true,
		"render_sidecars":         true,
		"render_gateway_config":   true,
		"reconcile_revision":      true,
		"start_release_candidate": true,
		"run_health_gate":         true,
		"promote_release":         true,
		"rollback_release":        true,
		"garbage_collect_runtime": true,
	}

	if !validTypes[cmd.Type] {
		return nil, fmt.Errorf("unsupported command type %q for mesh driver", cmd.Type)
	}

	return &CommandResult{
		RequestID: cmd.RequestID,
		Status:    "dispatched",
		Output: map[string]any{
			"driver":  "distributed_mesh",
			"command": cmd.Type,
		},
	}, nil
}
