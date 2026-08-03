package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr        = "127.0.0.1:8080"
	defaultDatabaseURL = "postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable"
	defaultRedisURL    = "redis://127.0.0.1:6379/0"
	defaultAppOrigin   = "http://127.0.0.1:5173"
	defaultSessionTTL  = 30 * 24 * time.Hour
	defaultDBMaxConns  = int32(20)
	// Attempts per minute, per source IP, per replica, on the credential
	// endpoints. Set to a large value on a trusted private deployment.
	defaultAuthRateLimit = int32(10)
)

type Config struct {
	Addr             string
	DatabaseURL      string
	DatabaseMaxConns int32
	RedisURL         string
	CookieSecure     bool
	SessionTTL       time.Duration
	AppOrigin        string
	StaticDir        string
	AuthRateLimit    int
	// LocalesDir holds one <tag>.json translation catalog per language. Mount a
	// volume here to add or override translations without rebuilding the image.
	LocalesDir string
}

func Load() (Config, error) {
	secure, err := envBool("ROLLBOARD_COOKIE_SECURE", false)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := envDuration("ROLLBOARD_SESSION_TTL", defaultSessionTTL)
	if err != nil {
		return Config{}, err
	}
	databaseMaxConns, err := envPositiveInt32("ROLLBOARD_DATABASE_MAX_CONNS", defaultDBMaxConns)
	if err != nil {
		return Config{}, err
	}
	authRateLimit, err := envPositiveInt32("ROLLBOARD_AUTH_RATE_LIMIT", defaultAuthRateLimit)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:             env("ROLLBOARD_ADDR", defaultAddr),
		DatabaseURL:      env("ROLLBOARD_DATABASE_URL", defaultDatabaseURL),
		DatabaseMaxConns: databaseMaxConns,
		RedisURL:         env("ROLLBOARD_REDIS_URL", defaultRedisURL),
		CookieSecure:     secure,
		SessionTTL:       sessionTTL,
		AppOrigin:        env("ROLLBOARD_APP_ORIGIN", defaultAppOrigin),
		StaticDir:        strings.TrimSpace(os.Getenv("ROLLBOARD_STATIC_DIR")),
		AuthRateLimit:    int(authRateLimit),
		LocalesDir:       strings.TrimSpace(os.Getenv("ROLLBOARD_LOCALES_DIR")),
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

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return parsed, nil
}

func envPositiveInt32(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int32(parsed), nil
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
