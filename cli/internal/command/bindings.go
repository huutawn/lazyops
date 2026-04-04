package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/initplan"
)

type bindingsArgs struct {
	Project     string
	RuntimeMode initplan.RuntimeMode
	TargetKind  string
	TargetRef   string
	Status      string
	Reuse       bool
}

func bindingsCommand() *Command {
	return &Command{
		Name:    "bindings",
		Summary: "List deployment bindings.",
		Usage:   "lazyops bindings [--project <project-id-or-slug>] [--runtime-mode <mode>] [--target-kind <instance|mesh|cluster>] [--target-ref <target-ref>] [--status <status>] [--reuse]",
		Run: withAuth(func(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
			bindingsArgs, err := parseBindingsArgs(args)
			if err != nil {
				return err
			}

			projectsResponse, err := fetchProjects(ctx, runtime, credential)
			if err != nil {
				return err
			}

			project, err := selectProjectForBindings(projectsResponse.Projects, bindingsArgs.Project)
			if err != nil {
				return err
			}
			if project == nil {
				runtime.Output.Warn("project selection pending; rerun `lazyops bindings --project <project-id-or-slug>`")
				for _, project := range projectsResponse.Projects {
					runtime.Output.Info("project option: %s (%s)", project.Name, project.Slug)
				}
				return nil
			}

			bindingsResponse, err := fetchBindings(ctx, runtime, credential, project.ID)
			if err != nil {
				return err
			}

			instancesResponse, err := fetchInstances(ctx, runtime, credential)
			if err != nil {
				return err
			}
			meshNetworksResponse, err := fetchMeshNetworks(ctx, runtime, credential)
			if err != nil {
				return err
			}
			clustersResponse, err := fetchClusters(ctx, runtime, credential)
			if err != nil {
				return err
			}

			summaries := initplan.SummarizeBindings(bindingsResponse.Bindings)
			targets := initplan.SummarizeTargets(instancesResponse.Instances, meshNetworksResponse.MeshNetworks, clustersResponse.Clusters)
			projectSummary := initplan.ProjectSummary{
				ID:            project.ID,
				UserID:        project.UserID,
				Slug:          project.Slug,
				Name:          project.Name,
				DefaultBranch: project.DefaultBranch,
			}
			summaries = initplan.AnnotateBindingsWithTargets(summaries, targets, &projectSummary)
			summaries = filterBindingsSummaries(summaries, bindingsArgs)
			runtime.Output.Success("deployment bindings loaded")
			runtime.Output.Info("transport mode: %s", runtime.Transport.Mode())
			runtime.Output.Info("project: %s (%s)", project.Name, project.Slug)
			printBindingsFilters(runtime, bindingsArgs)
			if len(summaries) == 0 {
				runtime.Output.Warn("no deployment bindings match the current filters")
				return nil
			}

			for _, binding := range summaries {
				runtime.Output.Info("binding %s -> %s (%s, %s, status=%s)", binding.Name, binding.TargetRef, binding.RuntimeMode, binding.TargetKind, binding.TargetStatus)
				if binding.Reusable {
					runtime.Output.Info("reuse with: lazyops init --project %s --runtime-mode %s --binding %s", project.Slug, binding.RuntimeMode, binding.ID)
				} else {
					runtime.Output.Warn("binding %s is not reusable yet because target status is %s", binding.Name, binding.TargetStatus)
				}
			}

			if bindingsArgs.Reuse {
				reusable := initplan.ReusableBindings(summaries)
				switch len(reusable) {
				case 0:
					runtime.Output.Warn("no reusable bindings match the current filters")
				case 1:
					runtime.Output.Success("reuse candidate selected: %s", reusable[0].Name)
					runtime.Output.Info("next: run `lazyops init --project %s --runtime-mode %s --binding %s`", project.Slug, reusable[0].RuntimeMode, reusable[0].ID)
				default:
					runtime.Output.Warn("multiple reusable bindings match; narrow filters with `--runtime-mode`, `--target-kind`, `--target-ref`, or `--status`")
				}
			}

			return nil
		}),
	}
}

func parseBindingsArgs(args []string) (bindingsArgs, error) {
	flagSet := flag.NewFlagSet("bindings", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	project := flagSet.String("project", "", "project id or slug")
	runtimeMode := flagSet.String("runtime-mode", "", "runtime mode: standalone, distributed-mesh, distributed-k3s")
	targetKind := flagSet.String("target-kind", "", "target kind: instance, mesh, or cluster")
	targetRef := flagSet.String("target-ref", "", "binding target_ref")
	status := flagSet.String("status", "", "target status filter")
	reuse := flagSet.Bool("reuse", false, "show reuse guidance for filtered bindings")

	if err := flagSet.Parse(args); err != nil {
		return bindingsArgs{}, errors.New("invalid bindings flags. next: use `lazyops bindings [--project <project-id-or-slug>] [--runtime-mode <mode>] [--target-kind <instance|mesh|cluster>] [--target-ref <target-ref>] [--status <status>] [--reuse]`")
	}
	if flagSet.NArg() > 0 {
		return bindingsArgs{}, fmt.Errorf("unexpected bindings arguments: %s. next: use `lazyops bindings [--project <project-id-or-slug>] [--runtime-mode <mode>] [--target-kind <instance|mesh|cluster>] [--target-ref <target-ref>] [--status <status>] [--reuse]`", strings.Join(flagSet.Args(), " "))
	}

	mode, err := initplan.ParseRuntimeMode(*runtimeMode)
	if err != nil {
		return bindingsArgs{}, err
	}

	normalizedTargetKind := strings.TrimSpace(*targetKind)
	if normalizedTargetKind != "" && normalizedTargetKind != "instance" && normalizedTargetKind != "mesh" && normalizedTargetKind != "cluster" {
		return bindingsArgs{}, fmt.Errorf("target kind %q is invalid. next: use `--target-kind instance`, `--target-kind mesh`, or `--target-kind cluster`", normalizedTargetKind)
	}

	return bindingsArgs{
		Project:     strings.TrimSpace(*project),
		RuntimeMode: mode,
		TargetKind:  normalizedTargetKind,
		TargetRef:   strings.TrimSpace(*targetRef),
		Status:      strings.TrimSpace(*status),
		Reuse:       *reuse,
	}, nil
}

func selectProjectForBindings(projects []contracts.Project, selector string) (*contracts.Project, error) {
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects are available. next: create a project or verify CLI auth")
	}

	selector = strings.TrimSpace(selector)
	if selector == "" {
		if len(projects) == 1 {
			project := projects[0]
			return &project, nil
		}
		return nil, nil
	}

	for _, project := range projects {
		if selector == project.ID || selector == project.Slug || selector == project.Name {
			selected := project
			return &selected, nil
		}
	}

	return nil, fmt.Errorf("project %q was not found. next: rerun `lazyops bindings --project <project-id-or-slug>` with one of the listed projects", selector)
}

func filterBindingsSummaries(summaries []initplan.BindingSummary, args bindingsArgs) []initplan.BindingSummary {
	filtered := summaries
	if args.RuntimeMode != "" {
		filtered = initplan.FilterBindingsByRuntimeMode(filtered, args.RuntimeMode)
	}
	filtered = initplan.FilterBindingsByTargetKind(filtered, args.TargetKind)
	filtered = initplan.FilterBindingsByTargetRef(filtered, args.TargetRef)
	filtered = initplan.FilterBindingsByStatus(filtered, args.Status)
	return filtered
}

func printBindingsFilters(runtime *Runtime, args bindingsArgs) {
	applied := []string{}
	if args.RuntimeMode != "" {
		applied = append(applied, "runtime_mode="+string(args.RuntimeMode))
	}
	if strings.TrimSpace(args.TargetKind) != "" {
		applied = append(applied, "target_kind="+args.TargetKind)
	}
	if strings.TrimSpace(args.TargetRef) != "" {
		applied = append(applied, "target_ref="+args.TargetRef)
	}
	if strings.TrimSpace(args.Status) != "" {
		applied = append(applied, "status="+args.Status)
	}
	if args.Reuse {
		applied = append(applied, "reuse=true")
	}

	if len(applied) == 0 {
		runtime.Output.Info("filters: none")
		return
	}

	runtime.Output.Info("filters: %s", strings.Join(applied, ", "))
}
