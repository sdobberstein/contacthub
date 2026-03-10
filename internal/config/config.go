// Package config loads and validates contacthub server configuration.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration for the contacthub server.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
	Admin    *AdminConfig   `yaml:"admin,omitempty"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Listen                      string   `yaml:"listen"`
	BaseURL                     string   `yaml:"base_url"`
	PathPrefix                  string   `yaml:"path_prefix"`
	TrustedProxies              []string `yaml:"trusted_proxies"`
	// WellKnownCacheMaxAgeSeconds controls the Cache-Control header on the
	// /.well-known/carddav redirect (RFC 6764 §5). Values > 0 produce
	// "max-age=<n>, public" (capped at 86400); <= 0 produces "no-cache, must-revalidate".
	WellKnownCacheMaxAgeSeconds int `yaml:"well_known_cache_max_age_seconds"`
}

// AuthConfig holds authentication and session settings.
type AuthConfig struct {
	Provider  string        `yaml:"provider"`
	Session   SessionConfig `yaml:"session"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// SessionConfig holds session lifetime settings.
type SessionConfig struct {
	MaxAge      int `yaml:"max_age"`
	IdleTimeout int `yaml:"idle_timeout"`
}

// RateLimitConfig holds rate limiting settings for auth endpoints.
type RateLimitConfig struct {
	MaxAttempts int `yaml:"max_attempts"`
	Window      int `yaml:"window"`
}

// DatabaseConfig holds SQLite database settings.
type DatabaseConfig struct {
	Path          string `yaml:"path"`
	EncryptionKey string `yaml:"encryption_key"`
}

// LogConfig holds structured logging settings.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// AdminConfig holds first-run admin bootstrap credentials.
type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:                      ":8080",
			TrustedProxies:              []string{"127.0.0.1", "::1"},
			WellKnownCacheMaxAgeSeconds: 3600,
		},
		Auth: AuthConfig{
			Provider: "local",
			Session: SessionConfig{
				MaxAge:      86400,
				IdleTimeout: 7200,
			},
			RateLimit: RateLimitConfig{
				MaxAttempts: 5,
				Window:      900,
			},
		},
		Database: DatabaseConfig{
			Path: "/data/contacthub.db",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load reads a YAML config file and applies CONTACTHUB_* environment overrides.
// path may be empty, in which case only defaults and env vars are used.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		f, err := os.Open(path) // #nosec G304 -- path is operator-supplied (CLI flag / env var), not user input
		if err != nil {
			return nil, fmt.Errorf("open config: %w", err)
		}
		defer f.Close() //nolint:errcheck // read-only file, close error not actionable

		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		if err := dec.Decode(cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	applyEnv(cfg)
	return cfg, nil
}

// applyEnv overrides config fields from CONTACTHUB_* env vars.
// Mapping: CONTACTHUB_SERVER_LISTEN -> cfg.Server.Listen, etc.
func applyEnv(cfg *Config) {
	if v := env("SERVER_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if v := env("SERVER_BASE_URL"); v != "" {
		cfg.Server.BaseURL = v
	}
	if v := env("SERVER_PATH_PREFIX"); v != "" {
		cfg.Server.PathPrefix = v
	}
	if v := env("DATABASE_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := env("DATABASE_ENCRYPTION_KEY"); v != "" {
		cfg.Database.EncryptionKey = v
	}
	if v := env("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := env("LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := env("AUTH_PROVIDER"); v != "" {
		cfg.Auth.Provider = v
	}
	if v := env("ADMIN_USER"); v != "" {
		if cfg.Admin == nil {
			cfg.Admin = &AdminConfig{}
		}
		cfg.Admin.Username = v
	}
	if v := env("ADMIN_PASSWORD"); v != "" {
		if cfg.Admin == nil {
			cfg.Admin = &AdminConfig{}
		}
		cfg.Admin.Password = v
	}
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv("CONTACTHUB_" + key))
}
