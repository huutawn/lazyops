package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/redact"
	"lazyops-cli/internal/repo"
	"lazyops-cli/internal/transport"
)

func NewRootCommand() *Command {
	return &Command{
		Name:    "lazyops",
		Summary: "LazyOps local operator CLI.",
		Usage:   "lazyops <command>",
		Subcommands: []*Command{
			loginCommand(),
			logoutCommand(),
			initCommand(),
			linkCommand(),
			authFixtureCommand(
				"doctor",
				"Validate local onboarding and deploy contract health.",
				"lazyops doctor",
				"Day 2 scaffold: doctor uses a mock-only preview contract until the backend API is locked.",
				transport.Request{Method: "GET", Path: "/mock/v1/doctor", Query: map[string]string{"project": "prj_demo"}},
			),
			authFixtureCommand(
				"status",
				"Show a thin runtime summary.",
				"lazyops status",
				"Day 2 scaffold: status uses a mock-only preview contract until the backend contract is locked.",
				transport.Request{Method: "GET", Path: "/mock/v1/status", Query: map[string]string{"project": "prj_demo"}},
			),
			authFixtureCommand(
				"bindings",
				"List deployment bindings.",
				"lazyops bindings",
				"Day 2 scaffold: bindings is wired to the mock transport.",
				transport.Request{Method: "GET", Path: "/api/v1/projects/prj_demo/deployment-bindings"},
			),
			logsCommand(),
			tracesCommand(),
			{
				Name:    "tunnel",
				Summary: "Open an optional debug tunnel.",
				Usage:   "lazyops tunnel <db|tcp>",
				Subcommands: []*Command{
					authFixtureCommand(
						"db",
						"Open a debug database tunnel.",
						"lazyops tunnel db",
						"Day 2 scaffold: tunnel db is wired to the mock transport.",
						transport.Request{Method: "POST", Path: "/api/v1/tunnels/db/sessions"},
					),
					authFixtureCommand(
						"tcp",
						"Open a debug TCP tunnel.",
						"lazyops tunnel tcp",
						"Day 2 scaffold: tunnel tcp is wired to the mock transport.",
						transport.Request{Method: "POST", Path: "/api/v1/tunnels/tcp/sessions"},
					),
				},
			},
		},
	}
}

func initCommand() *Command {
	return &Command{
		Name:    "init",
		Summary: "Initialize the repository into a valid LazyOps deploy contract.",
		Usage:   "lazyops init",
		Run: withAuth(func(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
			scanResult, err := repo.Scan(".")
			if err != nil {
				if errors.Is(err, repo.ErrRepoRootNotFound) {
					return fmt.Errorf("could not find the repository root. next: run `lazyops init` from inside a git repository")
				}
				return fmt.Errorf("could not scan the local repository. next: verify the working tree is readable and retry `lazyops init`: %w", err)
			}

			detectionResult, err := repo.DetectServices(scanResult)
			if err != nil {
				return fmt.Errorf("could not turn scan results into service candidates. next: fix the detected service layout and retry `lazyops init`: %w", err)
			}

			printRepoScanPreview(runtime, scanResult, detectionResult)

			requests := []transport.Request{
				{Method: "GET", Path: "/api/v1/projects"},
				{Method: "GET", Path: "/api/v1/instances"},
				{Method: "GET", Path: "/api/v1/mesh-networks"},
				{Method: "GET", Path: "/api/v1/clusters"},
			}

			return runAuthorizedSequence(ctx, runtime, "Day 2 scaffold: init dependencies are wired to the transport abstraction.", credential, requests...)
		}),
	}
}

func printRepoScanPreview(runtime *Runtime, scanResult repo.RepoScanResult, detectionResult repo.DetectionResult) {
	runtime.Output.Success("repo scan complete")
	runtime.Output.Info("repo root: %s", scanResult.RepoRoot)
	runtime.Output.Info("repo layout: %s", scanResult.LayoutLabel())

	if len(detectionResult.Candidates) == 0 {
		runtime.Output.Warn("no service markers found yet; expected one of package.json, go.mod, requirements.txt, or Dockerfile")
		return
	}

	for _, service := range detectionResult.Candidates {
		runtime.Output.Info("service %s -> %s (%s)", service.Name, service.Path, strings.Join(signalNames(service.Signals), ", "))
		if strings.TrimSpace(service.StartHint) != "" {
			runtime.Output.Info("start hint for %s: %s", service.Name, service.StartHint)
		}
		if strings.TrimSpace(service.Healthcheck.Path) != "" {
			if service.Healthcheck.Port > 0 {
				runtime.Output.Info("health hint for %s: %s on %d", service.Name, service.Healthcheck.Path, service.Healthcheck.Port)
			} else {
				runtime.Output.Info("health hint for %s: %s", service.Name, service.Healthcheck.Path)
			}
		}
		for _, warning := range service.Warnings {
			runtime.Output.Warn("service %s: %s", service.Name, warning)
		}
	}
}

func signalNames(signals []repo.ServiceSignal) []string {
	names := make([]string, 0, len(signals))
	for _, signal := range signals {
		names = append(names, string(signal))
	}
	return names
}

func linkCommand() *Command {
	return &Command{
		Name:    "link",
		Summary: "Connect the local repo to a project and GitHub App installation.",
		Usage:   "lazyops link",
		Run: withAuth(func(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
			return renderAuthorizedRequest(ctx, runtime, "Day 2 scaffold: link is wired to the transport abstraction.", credential, transport.Request{
				Method: "POST",
				Path:   "/api/v1/projects/prj_demo/repo-link",
			})
		}),
	}
}

func logsCommand() *Command {
	return &Command{
		Name:    "logs",
		Summary: "Inspect service logs.",
		Usage:   "lazyops logs <service>",
		Run: withAuth(func(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: lazyops logs <service>")
			}

			service := strings.TrimSpace(args[0])
			if service == "" {
				return fmt.Errorf("service name is required")
			}

			return renderAuthorizedRequest(ctx, runtime, "Day 2 scaffold: logs is wired to the transport abstraction.", credential, transport.Request{
				Method: "GET",
				Path:   "/ws/logs/stream",
				Query: map[string]string{
					"service": service,
				},
			})
		}),
	}
}

func tracesCommand() *Command {
	return &Command{
		Name:    "traces",
		Summary: "Inspect distributed request flow.",
		Usage:   "lazyops traces <correlation-id>",
		Run: withAuth(func(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: lazyops traces <correlation-id>")
			}

			correlationID := strings.TrimSpace(args[0])
			if correlationID == "" {
				return fmt.Errorf("correlation id is required")
			}

			return renderAuthorizedRequest(ctx, runtime, "Day 2 scaffold: traces is wired to the transport abstraction.", credential, transport.Request{
				Method: "GET",
				Path:   "/api/v1/traces/" + correlationID,
			})
		}),
	}
}

func fixtureCommand(name string, summary string, usage string, title string, request transport.Request) *Command {
	return &Command{
		Name:    name,
		Summary: summary,
		Usage:   usage,
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			return renderRequest(ctx, runtime, title, request)
		},
	}
}

func authFixtureCommand(name string, summary string, usage string, title string, request transport.Request) *Command {
	return &Command{
		Name:    name,
		Summary: summary,
		Usage:   usage,
		Run: withAuth(func(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
			return renderAuthorizedRequest(ctx, runtime, title, credential, request)
		}),
	}
}

func runSequence(ctx context.Context, runtime *Runtime, title string, requests ...transport.Request) error {
	spinner := runtime.SpinnerFactory.New()
	spinner.Start("loading mock contract previews")
	defer spinner.Stop("")

	runtime.Output.Success("%s", title)
	runtime.Output.Info("transport mode: %s", runtime.Transport.Mode())
	for _, request := range requests {
		response, err := runtime.Transport.Do(ctx, request)
		if err != nil {
			return err
		}

		printResponse(runtime, request, response)
	}

	return nil
}

func renderRequest(ctx context.Context, runtime *Runtime, title string, request transport.Request) error {
	spinner := runtime.SpinnerFactory.New()
	spinner.Start("loading mock contract preview")
	defer spinner.Stop("")

	response, err := runtime.Transport.Do(ctx, request)
	if err != nil {
		return err
	}

	runtime.Output.Success("%s", title)
	runtime.Output.Info("transport mode: %s", runtime.Transport.Mode())
	printResponse(runtime, request, response)
	return nil
}

func printResponse(runtime *Runtime, request transport.Request, response transport.Response) {
	runtime.Output.Print("")
	runtime.Output.Print("request: %s", request.Key())
	runtime.Output.Print("fixture: %s", response.FixtureName)
	runtime.Output.Print("status: %d", response.StatusCode)

	if len(response.Body) == 0 {
		return
	}

	runtime.Output.Print("%s", string(redact.PrettyJSON(response.Body)))
}
