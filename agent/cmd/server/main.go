package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"lazyops-agent/internal/app"
	"lazyops-agent/internal/config"
	appruntime "lazyops-agent/internal/runtime"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "compatibility-sidecar" {
		runCompatibilitySidecar()
		return
	}

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

func runCompatibilitySidecar() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := appruntime.LoadCompatibilitySidecarConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load compatibility sidecar config: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := appruntime.RunCompatibilitySidecar(ctx, logger, cfg); err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintf(os.Stderr, "run compatibility sidecar: %v\n", err)
		os.Exit(1)
	}
}
