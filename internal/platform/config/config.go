package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr             string
	DatabaseURL      string
	SessionTTL       time.Duration
	PasswordResetURL string
	SMTP             SMTP
}

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:             getenv("APP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		PasswordResetURL: getenv("PASSWORD_RESET_URL", "http://localhost:3000/reset-password"),
		SMTP: SMTP{
			Host:     os.Getenv("SMTP_HOST"),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     getenv("SMTP_FROM", "GovFac Report <no-reply@govfac.local>"),
		},
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

	if u, err := url.Parse(cfg.PasswordResetURL); err != nil || !u.IsAbs() {
		return Config{}, fmt.Errorf("PASSWORD_RESET_URL must be an absolute URL, got %q", cfg.PasswordResetURL)
	}

	rawPort := getenv("SMTP_PORT", "587")
	port, err := strconv.Atoi(rawPort)
	if err != nil || port <= 0 || port > 65535 {
		return Config{}, fmt.Errorf("invalid SMTP_PORT %q", rawPort)
	}
	cfg.SMTP.Port = port

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
