package status

import (
	"fmt"
	"strings"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/initplan"
	"lazyops-cli/internal/lazyyaml"
)

const AdapterName = "existing-api-composition/v1"

type Summary struct {
	Source           string
	Project          ProjectRef
	RuntimeMode      initplan.RuntimeMode
	DeclaredServices int
	Validation       ValidationState
	Binding          BindingState
	Topology         TopologyState
	Deployment       DeploymentState
}

type ProjectRef struct {
	ID   string
	Slug string
	Name string
}

type BindingState struct {
	State       string
	Name        string
	TargetRef   string
	RuntimeMode initplan.RuntimeMode
	TargetKind  string
}

type TopologyState struct {
	State  string
	Kind   string
	Name   string
	Status string
}

type DeploymentState struct {
	State    string
	Rollout  string
	Summary  string
	NextStep string
}

type ValidationState struct {
	State    string
	Summary  string
	NextStep string
}

type TargetSnapshot struct {
	ID     string
	Kind   string
	Name   string
	Status string
}

type Input struct {
	Contract   lazyyaml.DoctorMetadata
	Project    contracts.Project
	Binding    *contracts.DeploymentBinding
	Target     *TargetSnapshot
	Validation ValidationState
}

func BuildAdapterSummary(input Input) (Summary, error) {
	if err := input.Contract.ValidateDoctorContract(); err != nil {
		return Summary{}, err
	}
	if err := input.Project.Validate(); err != nil {
		return Summary{}, err
	}

	summary := Summary{
		Source:      AdapterName,
		RuntimeMode: input.Contract.RuntimeMode,
		Project: ProjectRef{
			ID:   input.Project.ID,
			Slug: input.Project.Slug,
			Name: input.Project.Name,
		},
		DeclaredServices: len(input.Contract.Services),
		Validation:       normalizeValidationState(input.Validation),
	}

	bindingState := buildBindingState(input.Contract, input.Binding)
	topologyState := buildTopologyState(input.Contract.RuntimeMode, input.Project, input.Binding, input.Target)
	deploymentState := buildDeploymentState(summary.Validation, bindingState, topologyState)

	summary.Binding = bindingState
	summary.Topology = topologyState
	summary.Deployment = deploymentState

	return summary, nil
}

func buildBindingState(contract lazyyaml.DoctorMetadata, binding *contracts.DeploymentBinding) BindingState {
	if binding == nil {
		return BindingState{
			State:       "missing",
			TargetRef:   contract.TargetRef,
			RuntimeMode: contract.RuntimeMode,
			TargetKind:  expectedTargetKind(contract.RuntimeMode),
		}
	}

	state := "attached"
	if binding.TargetRef != contract.TargetRef || binding.RuntimeMode != string(contract.RuntimeMode) {
		state = "drifted"
	}

	return BindingState{
		State:       state,
		Name:        binding.Name,
		TargetRef:   binding.TargetRef,
		RuntimeMode: initplan.RuntimeMode(binding.RuntimeMode),
		TargetKind:  binding.TargetKind,
	}
}

func buildTopologyState(mode initplan.RuntimeMode, project contracts.Project, binding *contracts.DeploymentBinding, target *TargetSnapshot) TopologyState {
	if binding == nil {
		return TopologyState{
			State: "pending",
			Kind:  expectedTargetKind(mode),
			Name:  "unbound",
		}
	}
	if target == nil {
		return TopologyState{
			State: "missing",
			Kind:  binding.TargetKind,
			Name:  binding.TargetRef,
		}
	}
	if targetHealthy(mode, target.Status) {
		return TopologyState{
			State:  "healthy",
			Kind:   target.Kind,
			Name:   target.Name,
			Status: target.Status,
		}
	}

	return TopologyState{
		State:  "degraded",
		Kind:   target.Kind,
		Name:   target.Name,
		Status: target.Status,
	}
}

func buildDeploymentState(validation ValidationState, binding BindingState, topology TopologyState) DeploymentState {
	switch {
	case validation.State == "failed":
		nextStep := validation.NextStep
		if strings.TrimSpace(nextStep) == "" {
			nextStep = "repair the deploy contract or rerun `lazyops init` before retrying `lazyops status`"
		}
		summary := validation.Summary
		if strings.TrimSpace(summary) == "" {
			summary = "control-plane validation failed"
		}
		return DeploymentState{
			State:    "blocked",
			Rollout:  "blocked",
			Summary:  summary,
			NextStep: nextStep,
		}
	case binding.State == "missing":
		return DeploymentState{
			State:    "blocked",
			Rollout:  "blocked",
			Summary:  "deploy contract is missing a compatible deployment binding",
			NextStep: "rerun `lazyops init` to create or reuse a compatible deployment binding",
		}
	case binding.State == "drifted":
		return DeploymentState{
			State:    "blocked",
			Rollout:  "blocked",
			Summary:  "deploy contract and bound target have drifted apart",
			NextStep: "rerun `lazyops init` to refresh lazyops.yaml and attach the correct deployment binding",
		}
	case topology.State == "pending":
		return DeploymentState{
			State:    "blocked",
			Rollout:  "blocked",
			Summary:  "deployment topology is not attached yet",
			NextStep: "rerun `lazyops init` and attach a deployment binding before deploying",
		}
	case topology.State == "missing":
		return DeploymentState{
			State:    "blocked",
			Rollout:  "blocked",
			Summary:  "deployment binding points to a target that no longer exists",
			NextStep: "rerun `lazyops init` and choose a valid target before the next deploy",
		}
	case topology.State == "ownership-mismatch":
		return DeploymentState{
			State:    "blocked",
			Rollout:  "blocked",
			Summary:  "target ownership no longer matches the selected project",
			NextStep: "choose a target owned by the project or recreate the deployment binding with `lazyops init`",
		}
	case topology.State == "degraded":
		return DeploymentState{
			State:    "degraded",
			Rollout:  "paused",
			Summary:  "deploy contract is attached, but the target is not ready",
			NextStep: "bring the target back online or choose a different binding before the next deploy",
		}
	default:
		return DeploymentState{
			State:    "ready",
			Rollout:  "idle",
			Summary:  "deploy contract, binding, and topology are aligned",
			NextStep: "push or open a pull request to trigger deployment through GitHub",
		}
	}
}

func normalizeValidationState(validation ValidationState) ValidationState {
	state := strings.TrimSpace(validation.State)
	if state == "" {
		validation.State = "unavailable"
		return validation
	}
	validation.State = state
	return validation
}

func (binding BindingState) Detail() string {
	if strings.TrimSpace(binding.Name) == "" {
		return fmt.Sprintf("%s target_ref %s", binding.State, binding.TargetRef)
	}
	return fmt.Sprintf("%s (%s -> %s)", binding.State, binding.Name, binding.TargetRef)
}

func (validation ValidationState) Detail() string {
	if strings.TrimSpace(validation.Summary) == "" {
		return validation.State
	}
	return fmt.Sprintf("%s (%s)", validation.State, validation.Summary)
}

func (topology TopologyState) Detail() string {
	if strings.TrimSpace(topology.Status) == "" {
		return fmt.Sprintf("%s (%s %s)", topology.State, topology.Kind, topology.Name)
	}
	return fmt.Sprintf("%s (%s %s, status=%s)", topology.State, topology.Kind, topology.Name, topology.Status)
}

func expectedTargetKind(mode initplan.RuntimeMode) string {
	switch mode {
	case initplan.RuntimeModeStandalone:
		return "instance"
	case initplan.RuntimeModeDistributedMesh:
		return "mesh"
	case initplan.RuntimeModeDistributedK3s:
		return "cluster"
	default:
		return "target"
	}
}

func targetHealthy(mode initplan.RuntimeMode, status string) bool {
	switch mode {
	case initplan.RuntimeModeDistributedK3s:
		return strings.EqualFold(status, "registered") ||
			strings.EqualFold(status, "online") ||
			strings.EqualFold(status, "available")
	default:
		return strings.EqualFold(status, "online")
	}
}
