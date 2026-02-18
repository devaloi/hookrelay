// Package config provides configuration management for the webhook relay service.
package config

import (
	"os"
	"strconv"
	"time"

	"github.com/devaloi/hookrelay/internal/domain"
)

// Config holds all configuration for the application.
type Config struct {
	// Server settings
	Port string

	// Database settings
	DatabaseURL string

	// Worker settings
	MaxRetries       int
	WorkerInterval   time.Duration
	DeliveryTimeout  time.Duration
	RetryBackoffBase int
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "./hookrelay.db"),
		MaxRetries:       getEnvInt("MAX_RETRIES", 5),
		WorkerInterval:   getEnvDuration("WORKER_INTERVAL", 5*time.Second),
		DeliveryTimeout:  getEnvDuration("DELIVERY_TIMEOUT", 30*time.Second),
		RetryBackoffBase: getEnvInt("RETRY_BACKOFF_BASE", domain.RetryBackoffBase),
	}
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvInt returns an integer environment variable or a default value.
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvDuration returns a duration environment variable or a default value.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
