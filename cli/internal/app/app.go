package app

import (
	"context"
	"io"

	"lazyops-cli/internal/command"
	"lazyops-cli/internal/transport"
	"lazyops-cli/internal/ui"
)

type App struct {
	root    *command.Command
	runtime *command.Runtime
}

func NewFromEnv(stdout io.Writer, stderr io.Writer) (*App, error) {
	cfg := LoadConfigFromEnv()
	output := ui.NewConsoleOutput(stdout, stderr)
	spinnerFactory := ui.NewSpinnerFactory(stderr)

	var client transport.Transport
	if cfg.UseMockTransport() {
		client = transport.NewMockTransport(transport.DefaultFixtures())
	} else {
		client = transport.NewHTTPTransport(cfg.APIBaseURL, nil)
	}

	runtime := &command.Runtime{
		Output:         output,
		SpinnerFactory: spinnerFactory,
		Transport:      client,
		Config: command.RuntimeConfig{
			TransportMode: cfg.TransportMode,
			APIBaseURL:    cfg.APIBaseURL,
		},
	}

	return &App{
		root:    command.NewRootCommand(),
		runtime: runtime,
	}, nil
}

func (a *App) Run(ctx context.Context, args []string) error {
	return a.root.Execute(ctx, a.runtime, args)
}
