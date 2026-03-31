package main

import (
	"lazyops-server/internal/api"
	"lazyops-server/internal/bootstrap"
	"lazyops-server/internal/config"
	"lazyops-server/pkg/logger"
)

func main() {
	cfg := config.Load()
	logger.Setup(cfg.App.Environment)

	app, err := bootstrap.NewApplication(cfg)
	if err != nil {
		logger.Fatal("bootstrap failed", "error", err)
	}

	router := api.NewRouter(app)

	logger.Info("server listening", "address", cfg.ServerAddress())
	if err := router.Run(cfg.ServerAddress()); err != nil {
		logger.Fatal("server stopped", "error", err)
	}
}
