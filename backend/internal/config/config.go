package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultAddr        = "127.0.0.1:8080"
	defaultDatabaseURL = "postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable"
	defaultRedisURL    = "redis://127.0.0.1:6379/0"
	defaultAppOrigin   = "http://127.0.0.1:5173"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	RedisURL     string
	CookieSecure bool
	AppOrigin    string
	StaticDir    string
}

func Load() (Config, error) {
	secure, err := envBool("ROLLBOARD_COOKIE_SECURE", false)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:         env("ROLLBOARD_ADDR", defaultAddr),
		DatabaseURL:  env("ROLLBOARD_DATABASE_URL", defaultDatabaseURL),
		RedisURL:     env("ROLLBOARD_REDIS_URL", defaultRedisURL),
		CookieSecure: secure,
		AppOrigin:    env("ROLLBOARD_APP_ORIGIN", defaultAppOrigin),
		StaticDir:    strings.TrimSpace(os.Getenv("ROLLBOARD_STATIC_DIR")),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("ROLLBOARD_ADDR must not be empty")
	}
	if err := validateURL("ROLLBOARD_DATABASE_URL", c.DatabaseURL, "postgres", "postgresql"); err != nil {
		return err
	}
	if err := validateURL("ROLLBOARD_REDIS_URL", c.RedisURL, "redis", "rediss"); err != nil {
		return err
	}
	return validateURL("ROLLBOARD_APP_ORIGIN", c.AppOrigin, "http", "https")
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func validateURL(name, rawURL string, schemes ...string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", name)
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return nil
		}
	}
	return fmt.Errorf("%s must use one of: %s", name, strings.Join(schemes, ", "))
}
