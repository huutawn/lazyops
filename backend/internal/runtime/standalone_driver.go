package runtime

import (
	"context"
	"fmt"
	"time"
)

type CommandDispatcher interface {
	DispatchCommand(ctx context.Context, agentID string, cmd AgentCommand) (*CommandResult, error)
}

type AgentHub interface {
	IsConnected(agentID string) bool
	ListConnectedAgentIDs() []string
}

type StandaloneDriver struct {
	dispatcher     CommandDispatcher
	agentHub       AgentHub
	commandTimeout time.Duration
}

func NewStandaloneDriver() *StandaloneDriver {
	return &StandaloneDriver{
		commandTimeout: 5 * time.Minute,
	}
}

func (d *StandaloneDriver) WithCommandDispatcher(dispatcher CommandDispatcher, hub AgentHub) *StandaloneDriver {
	d.dispatcher = dispatcher
	d.agentHub = hub
	return d
}

func (d *StandaloneDriver) Mode() string {
	return RuntimeModeStandalone
}

func (d *StandaloneDriver) Info() DriverInfo {
	return DriverInfo{
		Mode: RuntimeModeStandalone,
		Capabilities: []DriverCapability{
			CapabilityLocalProcess,
			CapabilityGatewayManagement,
			CapabilityHealthProbing,
			CapabilityScaleToZero,
		},
	}
}

func (d *StandaloneDriver) ValidateTarget(ctx context.Context, target TargetSpec) error {
	if target.TargetKind != "instance" {
		return fmt.Errorf("standalone driver requires target kind %q, got %q", "instance", target.TargetKind)
	}
	if target.TargetID == "" {
		return fmt.Errorf("standalone driver requires non-empty target_id")
	}
	return nil
}

func (d *StandaloneDriver) PlanRollout(ctx context.Context, req RolloutRequest) (*RolloutPlan, error) {
	return &RolloutPlan{
		Steps: []RolloutStep{
			{
				Kind: "prepare_workspace",
				Command: AgentCommand{
					Type:      "prepare_release_workspace",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "render_sidecars",
				Command: AgentCommand{
					Type:      "render_sidecars",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "render_gateway_config",
				Command: AgentCommand{
					Type:      "render_gateway_config",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "reconcile",
				Command: AgentCommand{
					Type:      "reconcile_revision",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "provision_internal_services",
				Command: AgentCommand{
					Type:      "provision_internal_services",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "start_candidate",
				Command: AgentCommand{
					Type:      "start_release_candidate",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "health_gate",
				Command: AgentCommand{
					Type:      "run_health_gate",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
			{
				Kind: "promote",
				Command: AgentCommand{
					Type:      "promote_release",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   req.RevisionPayload,
				},
			},
		},
		RuntimeMode: RuntimeModeStandalone,
		TargetKind:  "instance",
	}, nil
}

func (d *StandaloneDriver) ExecuteCommand(ctx context.Context, cmd AgentCommand) (*CommandResult, error) {
	if cmd.Type == "" {
		return nil, fmt.Errorf("command type is required")
	}

	validTypes := map[string]bool{
		"prepare_release_workspace":   true,
		"render_sidecars":             true,
		"render_gateway_config":       true,
		"reconcile_revision":          true,
		"provision_internal_services": true,
		"start_release_candidate":     true,
		"run_health_gate":             true,
		"promote_release":             true,
		"rollback_release":            true,
		"garbage_collect_runtime":     true,
	}

	if !validTypes[cmd.Type] {
		return nil, fmt.Errorf("unsupported command type %q for standalone driver", cmd.Type)
	}

	if d.dispatcher != nil && d.agentHub != nil {
		agentIDs := d.agentHub.ListConnectedAgentIDs()
		if len(agentIDs) == 0 {
			return &CommandResult{
				RequestID: cmd.RequestID,
				Status:    "failed",
				Output: map[string]any{
					"driver":  "standalone",
					"command": cmd.Type,
					"reason":  "no connected agents found",
				},
			}, nil
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, d.commandTimeout)
		defer cancel()

		return d.dispatcher.DispatchCommand(timeoutCtx, agentIDs[0], cmd)
	}

	return &CommandResult{
		RequestID: cmd.RequestID,
		Status:    "dispatched",
		Output: map[string]any{
			"driver":  "standalone",
			"command": cmd.Type,
		},
	}, nil
}
