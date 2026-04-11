package main

import (
	"context"
	"os/signal"
	"syscall"

	"lazyops-server/internal/config"
	"lazyops-server/internal/pkg/buildworker"
	"lazyops-server/pkg/logger"
)

const (
	BuildVersion = "dev"
	BuildCommit  = "local"
)

func main() {
	cfg := config.Load()
	logger.Setup(cfg.App.Environment)

	logger.Info("🔨 lazyops-build-worker starting",
		"version", BuildVersion,
		"commit", BuildCommit,
		"registry", cfg.BuildWorker.RegistryHost,
		"workspace", cfg.BuildWorker.WorkspaceDir,
	)

	worker, err := buildworker.New(cfg)
	if err != nil {
		logger.Fatal("failed to create build worker", "error", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := worker.Run(ctx); err != nil {
			logger.Error("worker stopped with error", "error", err)
		}
		stop()
	}()

	<-ctx.Done()
	logger.Info("🔨 build worker shutting down")
	worker.Shutdown()
}
