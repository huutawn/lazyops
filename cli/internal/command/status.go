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

	metadata, err := readDoctorMetadata(repoRoot, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("lazyops.yaml was not found at the repo root. next: run `lazyops init` before `lazyops status`")
		}
		return fmt.Errorf("could not read lazyops.yaml. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}
	if err := metadata.ValidateDoctorContract(); err != nil {
		return fmt.Errorf("lazyops.yaml is incomplete. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}

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

	summary, err := lazystatus.BuildAdapterSummary(lazystatus.Input{
		Contract: metadata,
		Project:  project,
		Binding:  selectedBinding,
		Target:   targetSnapshot,
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
