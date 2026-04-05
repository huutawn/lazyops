package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/lazyyaml"
	"lazyops-cli/internal/repo"
	lazystatus "lazyops-cli/internal/status"
)

func statusCommand() *Command {
	return &Command{
		Name:    "status",
		Summary: "Show a thin runtime summary.",
		Usage:   "lazyops status",
		Run:     withAuth(runStatus),
	}
}

func runStatus(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	if err := parseStatusArgs(args); err != nil {
		return err
	}

	repoRoot, err := repo.FindRepoRoot(".")
	if err != nil {
		if errors.Is(err, repo.ErrRepoRootNotFound) {
			return fmt.Errorf("could not find the repository root. next: run `lazyops status` from inside a git repository")
		}
		return fmt.Errorf("could not determine the repository root. next: verify the working tree is readable and retry `lazyops status`: %w", err)
	}

	document, err := readDoctorDocument(repoRoot, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("lazyops.yaml was not found at the repo root. next: run `lazyops init` before `lazyops status`")
		}
		return fmt.Errorf("could not read lazyops.yaml. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}
	if err := document.Validate(); err != nil {
		return fmt.Errorf("lazyops.yaml is incomplete. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}
	metadata := doctorMetadataFromDocument(document)

	projectsResponse, err := fetchProjects(ctx, runtime, credential)
	if err != nil {
		return err
	}
	project, err := selectProjectForLink(projectsResponse.Projects, metadata.ProjectSlug, credential)
	if err != nil {
		return err
	}

	bindingsResponse, err := fetchBindings(ctx, runtime, credential, project.ID)
	if err != nil {
		return err
	}

	var selectedBinding *contracts.DeploymentBinding
	selectedBindingValue, bindingErr := selectBindingForLink(bindingsResponse.Bindings, metadata.LinkMetadata())
	if bindingErr == nil {
		selectedBinding = &selectedBindingValue
	}

	var targetSnapshot *lazystatus.TargetSnapshot
	if selectedBinding != nil {
		discovery, err := fetchTargetDiscovery(ctx, runtime, credential)
		if err != nil {
			return err
		}
		if target, ok := resolveTargetForBinding(*selectedBinding, discovery); ok {
			targetSnapshot = &lazystatus.TargetSnapshot{
				ID:     target.ID,
				Kind:   target.Kind,
				Name:   target.Name,
				Status: target.Status,
			}
		}
	}

	validationState := lazystatus.ValidationState{
		State:   "unavailable",
		Summary: "control-plane validation was not available; status is using adapter composition",
	}
	if validation, validationErr := fetchValidateLazyopsYAML(ctx, runtime, credential, project.ID, document); validationErr == nil {
		selectedBinding = &validation.DeploymentBinding
		targetSnapshot = &lazystatus.TargetSnapshot{
			ID:     validation.TargetSummary.ID,
			Kind:   validation.TargetSummary.Kind,
			Name:   validation.TargetSummary.Name,
			Status: validation.TargetSummary.Status,
		}
		validationState = lazystatus.ValidationState{
			State:   "validated",
			Summary: fmt.Sprintf("binding %s and target %s %s [%s] validated", validation.DeploymentBinding.Name, validation.TargetSummary.Kind, validation.TargetSummary.Name, validation.TargetSummary.Status),
		}
	} else {
		summary, nextStep := splitNextStep(validationErr.Error())
		switch {
		case statusValidationFallback(validationErr):
			if strings.TrimSpace(summary) != "" {
				validationState.Summary = summary
			}
			if strings.TrimSpace(nextStep) != "" {
				validationState.NextStep = nextStep
			}
		default:
			validationState = lazystatus.ValidationState{
				State:    "failed",
				Summary:  summary,
				NextStep: nextStep,
			}
		}
	}

	summary, err := lazystatus.BuildAdapterSummary(lazystatus.Input{
		Contract:   metadata,
		Project:    project,
		Binding:    selectedBinding,
		Target:     targetSnapshot,
		Validation: validationState,
	})
	if err != nil {
		return fmt.Errorf("could not build the status summary. next: verify lazyops.yaml and backend contracts, then retry `lazyops status`: %w", err)
	}

	printStatusSummary(runtime, summary)
	return nil
}

func parseStatusArgs(args []string) error {
	flagSet := flag.NewFlagSet("status", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	if err := flagSet.Parse(args); err != nil {
		return errors.New("invalid status flags. next: use `lazyops status`")
	}
	if flagSet.NArg() > 0 {
		return fmt.Errorf("unexpected status arguments: %s. next: use `lazyops status`", strings.Join(flagSet.Args(), " "))
	}
	return nil
}

func printStatusSummary(runtime *Runtime, summary lazystatus.Summary) {
	runtime.Output.Print("status summary")
	runtime.Output.Info("source: %s", summary.Source)
	runtime.Output.Info("project: %s (%s)", summary.Project.Name, summary.Project.Slug)
	runtime.Output.Info("runtime mode: %s", summary.RuntimeMode)
	runtime.Output.Info("declared services: %d", summary.DeclaredServices)
	switch summary.Validation.State {
	case "validated":
		runtime.Output.Success("control-plane: %s", summary.Validation.Detail())
	case "failed":
		runtime.Output.Error("control-plane: %s", summary.Validation.Detail())
	default:
		runtime.Output.Warn("control-plane: %s", summary.Validation.Detail())
		if strings.TrimSpace(summary.Validation.NextStep) != "" {
			runtime.Output.Info("control-plane next: %s", summary.Validation.NextStep)
		}
	}
	runtime.Output.Info("binding state: %s", summary.Binding.Detail())
	runtime.Output.Info("topology state: %s", summary.Topology.Detail())

	switch summary.Deployment.State {
	case "ready":
		runtime.Output.Success("deployment state: %s (%s)", summary.Deployment.State, summary.Deployment.Summary)
	case "degraded":
		runtime.Output.Warn("deployment state: %s (%s)", summary.Deployment.State, summary.Deployment.Summary)
	default:
		runtime.Output.Error("deployment state: %s (%s)", summary.Deployment.State, summary.Deployment.Summary)
	}

	runtime.Output.Info("rollout: %s", summary.Deployment.Rollout)
	runtime.Output.Info("next: %s", summary.Deployment.NextStep)
}

func doctorMetadataFromDocument(document lazyyaml.Document) lazyyaml.DoctorMetadata {
	metadata := lazyyaml.DoctorMetadata{
		ProjectSlug:        document.ProjectSlug,
		RuntimeMode:        document.RuntimeMode,
		TargetRef:          document.DeploymentBinding.TargetRef,
		Services:           make([]lazyyaml.DoctorService, 0, len(document.Services)),
		DependencyBindings: make([]lazyyaml.DoctorDependencyBinding, 0, len(document.DependencyBindings)),
	}
	for _, service := range document.Services {
		metadata.Services = append(metadata.Services, lazyyaml.DoctorService{
			Name: service.Name,
			Path: service.Path,
		})
	}
	for _, binding := range document.DependencyBindings {
		metadata.DependencyBindings = append(metadata.DependencyBindings, lazyyaml.DoctorDependencyBinding{
			Service:       binding.Service,
			Alias:         binding.Alias,
			TargetService: binding.TargetService,
			Protocol:      binding.Protocol,
			LocalEndpoint: binding.LocalEndpoint,
		})
	}
	return metadata
}

func statusValidationFallback(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "request failed with status 0") ||
		strings.Contains(message, "fixture not found") ||
		strings.Contains(message, "internal_error") ||
		strings.Contains(message, "validation route")
}
