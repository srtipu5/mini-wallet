// Package config loads the application settings from environment variables.
package config

import (
	"time"

	"github.com/srtipu5/mini-wallet/internal/utils"
)

type Config struct {
	DatabaseURL string
	Port        string

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// Load reads DATABASE_URL and PORT. Everything has a sensible default, so the
// API starts against the docker-compose Postgres with no configuration.
func Load() Config {
	return Config{
		DatabaseURL: utils.Getenv("DATABASE_URL", "postgres://wallet:wallet@localhost:5432/wallet?sslmode=disable"),
		Port:        utils.Getenv("PORT", "8080"),

		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   15 * time.Second,
	}
}
