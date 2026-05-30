package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsConfigAndEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	body := []byte(`
server:
  port: "9000"
  mode: release
  timezone: "UTC"
cloak:
  base_url: "http://from-file.example"
  api_key: "file-key"
drivers:
  default_timeout: 60
  platforms:
    - "115"
offline:
  store:
    driver: sqlite
    dsn: "./test.db"
rate_limit:
  enabled: true
  rps: 5
  burst: 10
`)
	if err := os.WriteFile(configPath, body, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CLOAK_BASE_URL", "http://from-env.example")
	t.Setenv("CLOAK_API_KEY", "env-key")
	t.Setenv("TZ", "Asia/Shanghai")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "9000" {
		t.Fatalf("server.port = %q, want %q", cfg.Server.Port, "9000")
	}
	if cfg.Server.Timezone != "Asia/Shanghai" {
		t.Fatalf("server.timezone = %q, want env override", cfg.Server.Timezone)
	}
	if cfg.Cloak.BaseURL != "http://from-env.example" {
		t.Fatalf("cloak.base_url = %q, want env override", cfg.Cloak.BaseURL)
	}
	if cfg.Cloak.APIKey != "env-key" {
		t.Fatalf("cloak.api_key = %q, want env override", cfg.Cloak.APIKey)
	}
	if !cfg.RateLimit.Enabled || cfg.RateLimit.RPS != 5 || cfg.RateLimit.Burst != 10 {
		t.Fatalf("rate limit = %+v, want config values", cfg.RateLimit)
	}
	if cfg.Offline.Store.Driver != "sqlite" || cfg.Offline.Store.DSN != "./test.db" {
		t.Fatalf("offline store = %+v, want sqlite ./test.db", cfg.Offline.Store)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Port != "8080" {
		t.Fatalf("default port = %q, want 8080", cfg.Server.Port)
	}
	if cfg.Cloak.BaseURL == "" {
		t.Fatalf("default cloak base url is empty")
	}
	if len(cfg.Drivers.Platforms) != 1 || cfg.Drivers.Platforms[0] != "115" {
		t.Fatalf("default platforms = %#v, want [115]", cfg.Drivers.Platforms)
	}
	if cfg.Offline.Store.Driver != "sqlite" {
		t.Fatalf("default offline store driver = %q, want sqlite", cfg.Offline.Store.Driver)
	}
	if cfg.Offline.Store.DSN == "" {
		t.Fatalf("default offline store dsn is empty")
	}
}
