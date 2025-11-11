package app

import (
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	ServerPort       string
	JWTSecret        string
	JWTExpirationSec int64
	DBHost           string
	DBPort           string
	DBUser           string
	DBPassword       string
	DBName           string
	DBSSLMode        string
	LogRetentionDays int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		JWTSecret:        getEnv("JWT_SIGNING_SECRET", "change-me-in-production"),
		JWTExpirationSec: 86400, // 24 hours
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "5432"),
		DBUser:           getEnv("DB_USER", "postgres"),
		DBPassword:       getEnv("DB_PASSWORD", "postgres"),
		DBName:           getEnv("DB_NAME", "agentdb"),
		DBSSLMode:        getEnv("DB_SSL_MODE", "disable"),
		LogRetentionDays: 7,
	}

	if cfg.JWTSecret == "change-me-in-production" {
		return nil, fmt.Errorf("JWT_SIGNING_SECRET must be set")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
