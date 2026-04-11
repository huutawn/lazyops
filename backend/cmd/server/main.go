package main

import (
	"lazyops-server/internal/api"
	"lazyops-server/internal/bootstrap"
	"lazyops-server/internal/config"
	"lazyops-server/pkg/logger"
)

// Build markers — updated by CI/CD or manually
const (
	BuildVersion = "dev"
	BuildCommit  = "local"
)

func main() {
	cfg := config.Load()
	logger.Setup(cfg.App.Environment)
	if err := cfg.Validate(); err != nil {
		logger.Fatal("invalid configuration", "error", err)
	}

	app, err := bootstrap.NewApplication(cfg)
	if err != nil {
		logger.Fatal("bootstrap failed", "error", err)
	}

	router := api.NewRouter(app)

	// Startup markers — visible in logs to confirm binary version
	logger.Info("🚀 lazyops-server starting",
		"version", BuildVersion,
		"commit", BuildCommit,
		"address", cfg.ServerAddress(),
		"ssh_install_timeout", "200s",
	)

	if err := router.Run(cfg.ServerAddress()); err != nil {
		logger.Fatal("server stopped", "error", err)
	}
}
