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
	Project string
}

func bindingsCommand() *Command {
	return &Command{
		Name:    "bindings",
		Summary: "List deployment bindings.",
		Usage:   "lazyops bindings [--project <project-id-or-slug>]",
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

			summaries := initplan.SummarizeBindings(bindingsResponse.Bindings)
			runtime.Output.Success("deployment bindings loaded")
			runtime.Output.Info("transport mode: %s", runtime.Transport.Mode())
			runtime.Output.Info("project: %s (%s)", project.Name, project.Slug)
			if len(summaries) == 0 {
				runtime.Output.Warn("no deployment bindings exist yet for this project")
				return nil
			}

			for _, binding := range summaries {
				runtime.Output.Info("binding %s -> %s (%s, %s)", binding.Name, binding.TargetRef, binding.RuntimeMode, binding.TargetKind)
			}

			return nil
		}),
	}
}

func parseBindingsArgs(args []string) (bindingsArgs, error) {
	flagSet := flag.NewFlagSet("bindings", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	project := flagSet.String("project", "", "project id or slug")

	if err := flagSet.Parse(args); err != nil {
		return bindingsArgs{}, errors.New("invalid bindings flags. next: use `lazyops bindings [--project <project-id-or-slug>]`")
	}
	if flagSet.NArg() > 0 {
		return bindingsArgs{}, fmt.Errorf("unexpected bindings arguments: %s. next: use `lazyops bindings [--project <project-id-or-slug>]`", strings.Join(flagSet.Args(), " "))
	}

	return bindingsArgs{Project: strings.TrimSpace(*project)}, nil
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
