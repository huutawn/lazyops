package app

import (
	"context"
	"io"

	"lazyops-cli/internal/command"
	"lazyops-cli/internal/credentials"
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
	credentialStore, err := credentials.NewStore(cfg.Credentials)
	if err != nil {
		return nil, err
	}

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
		Credentials:    credentialStore,
		Config: command.RuntimeConfig{
			TransportMode: cfg.TransportMode,
			APIBaseURL:    cfg.APIBaseURL,
			ServiceName:   cfg.ServiceName,
			AccountName:   cfg.AccountName,
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
