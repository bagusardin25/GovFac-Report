package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Addr        string
	DatabaseURL string
	SessionTTL  time.Duration
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:        getenv("APP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required (see .env.example)")
	}

	rawTTL := getenv("SESSION_TTL", "168h")
	ttl, err := time.ParseDuration(rawTTL)
	if err != nil || ttl <= 0 {
		return Config{}, fmt.Errorf("invalid SESSION_TTL %q", rawTTL)
	}
	cfg.SessionTTL = ttl

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
