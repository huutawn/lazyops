package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"lazyops-agent/internal/app"
	"lazyops-agent/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load agent config: %v\n", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bootstrap agent app: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintf(os.Stderr, "run agent: %v\n", err)
		os.Exit(1)
	}
}
