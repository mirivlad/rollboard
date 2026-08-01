package config

import "testing"

func TestLoadUsesSafeDefaults(t *testing.T) {
	t.Setenv("ROLLBOARD_ADDR", "")
	t.Setenv("ROLLBOARD_DATABASE_URL", "")
	t.Setenv("ROLLBOARD_REDIS_URL", "")
	t.Setenv("ROLLBOARD_COOKIE_SECURE", "")
	t.Setenv("ROLLBOARD_APP_ORIGIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q, want default", cfg.Addr)
	}
	if cfg.DatabaseURL != "postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want default", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://127.0.0.1:6379/0" {
		t.Errorf("RedisURL = %q, want default", cfg.RedisURL)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure = true, want false")
	}
}

func TestLoadRejectsInvalidCookieSecurity(t *testing.T) {
	t.Setenv("ROLLBOARD_COOKIE_SECURE", "not-a-bool")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid ROLLBOARD_COOKIE_SECURE error")
	}
}

func TestLoadRejectsInvalidServiceURL(t *testing.T) {
	t.Setenv("ROLLBOARD_DATABASE_URL", "not a URL")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid ROLLBOARD_DATABASE_URL error")
	}
}
