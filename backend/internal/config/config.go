package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App         AppConfig
	Server      ServerConfig
	Database    DatabaseConfig
	JWT         JWTConfig
	PAT         PATConfig
	GoogleOAuth GoogleOAuthConfig
	GitHubOAuth GitHubOAuthConfig
	GitHubApp   GitHubAppConfig
	Enrollment  EnrollmentConfig
	Security    SecurityConfig
	Seed        SeedConfig
	WebSocket   WebSocketConfig
}

type AppConfig struct {
	Name        string
	Environment string
}

type ServerConfig struct {
	Host           string
	Port           string
	GinMode        string
	RequestTimeout time.Duration
}

type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	Name         string
	SSLMode      string
	TimeZone     string
	MaxIdleConns int
	MaxOpenConns int
}

type JWTConfig struct {
	Secret    string
	Issuer    string
	ExpiresIn time.Duration
}

type PATConfig struct {
	ExpiresIn time.Duration
}

type GoogleOAuthConfig struct {
	Enabled            bool
	ClientID           string
	ClientSecret       string
	CallbackURL        string
	SuccessRedirectURL string
	FailureRedirectURL string
	StateTTL           time.Duration
}

type GitHubOAuthConfig struct {
	Enabled            bool
	ClientID           string
	ClientSecret       string
	CallbackURL        string
	SuccessRedirectURL string
	FailureRedirectURL string
	StateTTL           time.Duration
}

type GitHubAppConfig struct {
	AppID         string
	ClientID      string
	ClientSecret  string
	PrivateKey    string
	WebhookSecret string
	Name          string
	CallbackURL   string
	WebhookURL    string
	InstallURL    string
}

type EnrollmentConfig struct {
	BootstrapTokenTTL time.Duration
	AgentTokenTTL     time.Duration
}

type SecurityConfig struct {
	AllowedOrigins         []string
	RateLimitRPS           float64
	RateLimitBurst         int
	CLILoginRateLimitRPS   float64
	CLILoginRateLimitBurst int
}

type SeedConfig struct {
	AdminEmail    string
	AdminPassword string
	AdminName     string
}

type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	PingPeriod      time.Duration
	PongWait        time.Duration
}

func Load() Config {
	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production")

	return Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "lazyops-server"),
			Environment: getEnv("APP_ENV", "development"),
		},
		Server: ServerConfig{
			Host:           getEnv("SERVER_HOST", "0.0.0.0"),
			Port:           getEnv("SERVER_PORT", "8080"),
			GinMode:        getEnv("GIN_MODE", "debug"),
			RequestTimeout: getEnvAsDuration("SERVER_REQUEST_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "127.0.0.1"),
			Port:         getEnv("DB_PORT", "5432"),
			User:         getEnv("DB_USER", "postgres"),
			Password:     getEnv("DB_PASSWORD", "postgres"),
			Name:         getEnv("DB_NAME", "lazyops"),
			SSLMode:      getEnv("DB_SSLMODE", "disable"),
			TimeZone:     getEnv("DB_TIMEZONE", "Asia/Bangkok"),
			MaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
			MaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 50),
		},
		JWT: JWTConfig{
			Secret:    jwtSecret,
			Issuer:    getEnv("JWT_ISSUER", "lazyops-server"),
			ExpiresIn: getEnvAsDuration("JWT_EXPIRES_IN", 24*time.Hour),
		},
		PAT: PATConfig{
			ExpiresIn: getEnvAsDuration("PAT_EXPIRES_IN", 30*24*time.Hour),
		},
		GoogleOAuth: GoogleOAuthConfig{
			Enabled:            getEnvAsBool("GOOGLE_OAUTH_ENABLED", true),
			ClientID:           getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret:       getEnv("GOOGLE_CLIENT_SECRET", ""),
			CallbackURL:        getEnv("GOOGLE_CALLBACK_URL", ""),
			SuccessRedirectURL: getEnv("GOOGLE_OAUTH_SUCCESS_REDIRECT_URL", ""),
			FailureRedirectURL: getEnv("GOOGLE_OAUTH_FAILURE_REDIRECT_URL", ""),
			StateTTL:           getEnvAsDuration("GOOGLE_OAUTH_STATE_TTL", 10*time.Minute),
		},
		GitHubOAuth: GitHubOAuthConfig{
			Enabled:            getEnvAsBool("GITHUB_OAUTH_ENABLED", true),
			ClientID:           getEnv("GITHUB_CLIENT_ID", ""),
			ClientSecret:       getEnv("GITHUB_CLIENT_SECRET", ""),
			CallbackURL:        getEnv("GITHUB_CALLBACK_URL", ""),
			SuccessRedirectURL: getEnv("GITHUB_OAUTH_SUCCESS_REDIRECT_URL", ""),
			FailureRedirectURL: getEnv("GITHUB_OAUTH_FAILURE_REDIRECT_URL", ""),
			StateTTL:           getEnvAsDuration("GITHUB_OAUTH_STATE_TTL", 10*time.Minute),
		},
		GitHubApp: GitHubAppConfig{
			AppID:         getEnv("GITHUB_APP_ID", ""),
			ClientID:      getEnv("GITHUB_APP_CLIENT_ID", ""),
			ClientSecret:  getEnv("GITHUB_APP_CLIENT_SECRET", ""),
			PrivateKey:    getEnv("GITHUB_APP_PRIVATE_KEY", ""),
			WebhookSecret: getEnv("GITHUB_APP_WEBHOOK_SECRET", ""),
			Name:          getEnv("GITHUB_APP_NAME", ""),
			CallbackURL:   getEnv("GITHUB_APP_CALLBACK_URL", ""),
			WebhookURL:    getEnv("GITHUB_APP_WEBHOOK_URL", ""),
			InstallURL:    getEnv("GITHUB_APP_INSTALL_URL", ""),
		},
		Enrollment: EnrollmentConfig{
			BootstrapTokenTTL: getEnvAsDuration("BOOTSTRAP_TOKEN_TTL", 15*time.Minute),
			AgentTokenTTL:     getEnvAsDuration("AGENT_TOKEN_TTL", 30*24*time.Hour),
		},
		Security: SecurityConfig{
			AllowedOrigins:         getEnvAsSlice("ALLOWED_ORIGINS", []string{"*"}),
			RateLimitRPS:           getEnvAsFloat("RATE_LIMIT_RPS", 10),
			RateLimitBurst:         getEnvAsInt("RATE_LIMIT_BURST", 20),
			CLILoginRateLimitRPS:   getEnvAsFloat("CLI_LOGIN_RATE_LIMIT_RPS", 0.5),
			CLILoginRateLimitBurst: getEnvAsInt("CLI_LOGIN_RATE_LIMIT_BURST", 3),
		},
		Seed: SeedConfig{
			AdminEmail:    getEnv("SEED_ADMIN_EMAIL", "admin@lazyops.local"),
			AdminPassword: getEnv("SEED_ADMIN_PASSWORD", "ChangeMe123!"),
			AdminName:     getEnv("SEED_ADMIN_NAME", "System Admin"),
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:  getEnvAsInt("WS_READ_BUFFER_SIZE", 1024),
			WriteBufferSize: getEnvAsInt("WS_WRITE_BUFFER_SIZE", 1024),
			PingPeriod:      getEnvAsDuration("WS_PING_PERIOD", 30*time.Second),
			PongWait:        getEnvAsDuration("WS_PONG_WAIT", 60*time.Second),
		},
	}
}

func (c Config) ServerAddress() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
		c.Database.TimeZone,
	)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsFloat(key string, fallback float64) float64 {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsSlice(key string, fallback []string) []string {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) == 0 {
		return fallback
	}
	return items
}

func getEnvAsBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(getEnv(key, "")))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
