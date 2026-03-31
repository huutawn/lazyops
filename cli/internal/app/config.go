package app

import (
	"os"
	"path/filepath"
	"strings"

	"lazyops-cli/internal/credentials"
)

const (
	defaultTransportMode = "mock"
	defaultAPIBaseURL    = "http://127.0.0.1:8080"
	defaultServiceName   = "lazyops-cli"
	defaultAccountName   = "default"
)

type Config struct {
	TransportMode string
	APIBaseURL    string
	ServiceName   string
	AccountName   string
	Credentials   credentials.StoreConfig
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

	serviceName := strings.TrimSpace(os.Getenv("LAZYOPS_KEYCHAIN_SERVICE"))
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	accountName := strings.TrimSpace(os.Getenv("LAZYOPS_KEYCHAIN_ACCOUNT"))
	if accountName == "" {
		accountName = defaultAccountName
	}

	credentialsPath := strings.TrimSpace(os.Getenv("LAZYOPS_CREDENTIALS_FILE"))
	if credentialsPath == "" {
		credentialsPath = credentials.DefaultCredentialsPath()
	} else {
		credentialsPath = filepath.Clean(credentialsPath)
	}

	return Config{
		TransportMode: transportMode,
		APIBaseURL:    apiBaseURL,
		ServiceName:   serviceName,
		AccountName:   accountName,
		Credentials: credentials.StoreConfig{
			Service:         serviceName,
			Account:         accountName,
			CredentialsPath: credentialsPath,
		},
	}
}

func (c Config) UseMockTransport() bool {
	return strings.EqualFold(c.TransportMode, "mock")
}
