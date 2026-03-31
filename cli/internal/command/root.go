package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"lazyops-cli/internal/transport"
)

func NewRootCommand() *Command {
	return &Command{
		Name:    "lazyops",
		Summary: "LazyOps local operator CLI scaffold for Day 2.",
		Usage:   "lazyops <command>",
		Subcommands: []*Command{
			fixtureCommand(
				"login",
				"Authenticate and store CLI identity.",
				"lazyops login",
				"Day 2 scaffold: login is wired to the transport abstraction.",
				transport.Request{Method: "POST", Path: "/api/v1/auth/cli-login"},
			),
			fixtureCommand(
				"logout",
				"Revoke or clear the local CLI session.",
				"lazyops logout",
				"Day 2 scaffold: logout is wired to the transport abstraction.",
				transport.Request{Method: "POST", Path: "/api/v1/auth/pat/revoke"},
			),
			initCommand(),
			linkCommand(),
			fixtureCommand(
				"doctor",
				"Validate local onboarding and deploy contract health.",
				"lazyops doctor",
				"Day 2 scaffold: doctor uses a mock-only preview contract until the backend API is locked.",
				transport.Request{Method: "GET", Path: "/mock/v1/doctor", Query: map[string]string{"project": "prj_demo"}},
			),
			fixtureCommand(
				"status",
				"Show a thin runtime summary.",
				"lazyops status",
				"Day 2 scaffold: status uses a mock-only preview contract until the backend contract is locked.",
				transport.Request{Method: "GET", Path: "/mock/v1/status", Query: map[string]string{"project": "prj_demo"}},
			),
			fixtureCommand(
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
					fixtureCommand(
						"db",
						"Open a debug database tunnel.",
						"lazyops tunnel db",
						"Day 2 scaffold: tunnel db is wired to the mock transport.",
						transport.Request{Method: "POST", Path: "/api/v1/tunnels/db/sessions"},
					),
					fixtureCommand(
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
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			requests := []transport.Request{
				{Method: "GET", Path: "/api/v1/projects"},
				{Method: "GET", Path: "/api/v1/instances"},
				{Method: "GET", Path: "/api/v1/mesh-networks"},
				{Method: "GET", Path: "/api/v1/clusters"},
			}

			return runSequence(ctx, runtime, "Day 2 scaffold: init dependencies are wired to the transport abstraction.", requests...)
		},
	}
}

func linkCommand() *Command {
	return &Command{
		Name:    "link",
		Summary: "Connect the local repo to a project and GitHub App installation.",
		Usage:   "lazyops link",
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			return renderRequest(ctx, runtime, "Day 2 scaffold: link is wired to the transport abstraction.", transport.Request{
				Method: "POST",
				Path:   "/api/v1/projects/prj_demo/repo-link",
			})
		},
	}
}

func logsCommand() *Command {
	return &Command{
		Name:    "logs",
		Summary: "Inspect service logs.",
		Usage:   "lazyops logs <service>",
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: lazyops logs <service>")
			}

			service := strings.TrimSpace(args[0])
			if service == "" {
				return fmt.Errorf("service name is required")
			}

			return renderRequest(ctx, runtime, "Day 2 scaffold: logs is wired to the transport abstraction.", transport.Request{
				Method: "GET",
				Path:   "/ws/logs/stream",
				Query: map[string]string{
					"service": service,
				},
			})
		},
	}
}

func tracesCommand() *Command {
	return &Command{
		Name:    "traces",
		Summary: "Inspect distributed request flow.",
		Usage:   "lazyops traces <correlation-id>",
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: lazyops traces <correlation-id>")
			}

			correlationID := strings.TrimSpace(args[0])
			if correlationID == "" {
				return fmt.Errorf("correlation id is required")
			}

			return renderRequest(ctx, runtime, "Day 2 scaffold: traces is wired to the transport abstraction.", transport.Request{
				Method: "GET",
				Path:   "/api/v1/traces/" + correlationID,
			})
		},
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

	var indented bytes.Buffer
	if err := json.Indent(&indented, response.Body, "", "  "); err != nil {
		runtime.Output.Print("%s", string(response.Body))
		return
	}

	runtime.Output.Print("%s", indented.String())
}
