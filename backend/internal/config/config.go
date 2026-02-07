package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	JWT        JWTConfig
	Google     GoogleConfig
	CORS       CORSConfig
	OpenSearch OpenSearchConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	Secret        string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
}

type CORSConfig struct {
	AllowedOrigins []string
}

type OpenSearchConfig struct {
	Endpoint string // OpenSearch cluster URL (e.g. https://search-xxx.us-east-1.es.amazonaws.com)
	Index    string // Index name (default: "papers")
	Username string // For fine-grained access control
	Password string
	Enabled  bool // Whether to use OpenSearch for search (falls back to PG if false)
}

func Load() *Config {
	osEndpoint := getEnv("OPENSEARCH_URL", "")
	return &Config{
		Server: ServerConfig{
			Port:         getEnvMulti([]string{"PORT", "SERVER_PORT"}, "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", "postgres://paper:paper@localhost:5432/paper?sslmode=disable"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", "your-super-secret-jwt-key"),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", "your-super-secret-refresh-key"),
			AccessExpiry:  getDurationEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshExpiry: getDurationEnv("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		Google: GoogleConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		},
		CORS: CORSConfig{
			AllowedOrigins: getSliceEnv("CORS_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
		},
		OpenSearch: OpenSearchConfig{
			Endpoint: osEndpoint,
			Index:    getEnv("OPENSEARCH_INDEX", "papers"),
			Username: getEnv("OPENSEARCH_USER", ""),
			Password: getEnv("OPENSEARCH_PASS", ""),
			Enabled:  osEndpoint != "",
		},
	}
}

func getEnvMulti(keys []string, defaultValue string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return defaultValue
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}
