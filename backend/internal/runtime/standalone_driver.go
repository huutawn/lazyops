package runtime

import (
	"context"
	"fmt"
)

type StandaloneDriver struct{}

func NewStandaloneDriver() *StandaloneDriver {
	return &StandaloneDriver{}
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
				Kind: "reconcile",
				Command: AgentCommand{
					Type:      "reconcile_revision",
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
					Payload: map[string]any{
						"revision_id": req.RevisionID,
					},
				},
			},
			{
				Kind: "health_gate",
				Command: AgentCommand{
					Type:      "run_health_gate",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   map[string]any{"revision_id": req.RevisionID},
				},
			},
			{
				Kind: "promote",
				Command: AgentCommand{
					Type:      "promote_release",
					ProjectID: req.ProjectID,
					Source:    "standalone_driver",
					Payload:   map[string]any{"revision_id": req.RevisionID},
				},
			},
		},
		RuntimeMode: RuntimeModeStandalone,
		TargetKind:  "instance",
	}, nil
}

func (d *StandaloneDriver) ExecuteCommand(ctx context.Context, cmd AgentCommand) (*CommandResult, error) {
	return &CommandResult{
		RequestID: cmd.RequestID,
		Status:    "dispatched",
	}, nil
}
