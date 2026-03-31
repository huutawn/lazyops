package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App       AppConfig
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Security  SecurityConfig
	Seed      SeedConfig
	WebSocket WebSocketConfig
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

type SecurityConfig struct {
	AllowedOrigins []string
	RateLimitRPS   float64
	RateLimitBurst int
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
			Secret:    getEnv("JWT_SECRET", "change-me-in-production"),
			Issuer:    getEnv("JWT_ISSUER", "lazyops-server"),
			ExpiresIn: getEnvAsDuration("JWT_EXPIRES_IN", 24*time.Hour),
		},
		Security: SecurityConfig{
			AllowedOrigins: getEnvAsSlice("ALLOWED_ORIGINS", []string{"*"}),
			RateLimitRPS:   getEnvAsFloat("RATE_LIMIT_RPS", 10),
			RateLimitBurst: getEnvAsInt("RATE_LIMIT_BURST", 20),
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
