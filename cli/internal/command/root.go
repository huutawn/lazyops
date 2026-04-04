package command

import (
	"context"

	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/redact"
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
			doctorCommand(),
			statusCommand(),
			bindingsCommand(),
			logsCommand(),
			tracesCommand(),
			tunnelCommand(),
			completionCommand(),
		},
	}
}

func initCommand() *Command {
	return &Command{
		Name:    "init",
		Summary: "Initialize the repository into a valid LazyOps deploy contract.",
		Usage:   "lazyops init [--project <project-id-or-slug>] [--runtime-mode <mode>] [--target <id|name>] [--binding <binding-id|name|target-ref> | --create-binding [--binding-name <name>]] [--magic-domain-provider <sslip.io|nip.io>] [--scale-to-zero] [--write [--overwrite]]",
		Run:     withAuth(runInit),
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
