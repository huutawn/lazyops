package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

const compatibilitySidecarConfigEnv = "LAZYOPS_COMPATIBILITY_CONFIG_B64"

func LoadCompatibilitySidecarConfig() (SidecarServiceConfig, error) {
	encoded := os.Getenv(compatibilitySidecarConfigEnv)
	if encoded == "" {
		return SidecarServiceConfig{}, fmt.Errorf("%s is required", compatibilitySidecarConfigEnv)
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return SidecarServiceConfig{}, fmt.Errorf("decode %s: %w", compatibilitySidecarConfigEnv, err)
	}
	var cfg SidecarServiceConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return SidecarServiceConfig{}, fmt.Errorf("decode compatibility sidecar config: %w", err)
	}
	return cfg, nil
}

func RunCompatibilitySidecar(ctx context.Context, logger *slog.Logger, cfg SidecarServiceConfig) error {
	proxy := NewSidecarProxy(logger)
	postgres := NewPostgresCompatAdapter(logger)

	for _, route := range cfg.ProxyRoutes {
		if err := proxy.StartRoute(ctx, route); err != nil {
			return err
		}
	}
	for _, contract := range cfg.ManagedDBAdapters {
		if err := postgres.Start(ctx, contract); err != nil {
			proxy.StopAll()
			return err
		}
	}

	<-ctx.Done()
	postgres.StopAll()
	proxy.StopAll()
	return nil
}
