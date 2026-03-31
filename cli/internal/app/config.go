package app

import (
	"os"
	"strings"
)

const (
	defaultTransportMode = "mock"
	defaultAPIBaseURL    = "http://127.0.0.1:8080"
)

type Config struct {
	TransportMode string
	APIBaseURL    string
}

func LoadConfigFromEnv() Config {
	transportMode := strings.TrimSpace(os.Getenv("LAZYOPS_TRANSPORT"))
	if transportMode == "" {
		transportMode = defaultTransportMode
	}

	apiBaseURL := strings.TrimSpace(os.Getenv("LAZYOPS_API_URL"))
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	return Config{
		TransportMode: transportMode,
		APIBaseURL:    apiBaseURL,
	}
}

func (c Config) UseMockTransport() bool {
	return strings.EqualFold(c.TransportMode, "mock")
}
