package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MinIOBindEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
server:
  port: "8080"
  mode: debug
database:
  host: from-yaml
  port: "5432"
  user: postgres
  password: postgres
  dbname: go_template
  sslmode: disable
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime: 30
jwt:
  secret: from-yaml-secret
  expire_hours: 72
scheduler:
  enabled: false
  timezone: "Asia/Shanghai"
snowflake:
  node_id: 1
rate_limit:
  enabled: false
  rps: 100
  burst: 200
websocket:
  enabled: false
storage:
  driver: minio
  minio:
    endpoint: "localhost:9000"
    access_key: "${MINIO_ACCESS_KEY}"
    secret_key: "${MINIO_SECRET_KEY}"
    bucket: go-template
    use_ssl: false
    public_url: "http://localhost:9000"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MINIO_ACCESS_KEY", "env-access")
	t.Setenv("MINIO_SECRET_KEY", "env-secret")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Storage.Minio.AccessKey != "env-access" {
		t.Fatalf("AccessKey = %q, want env-access", cfg.Storage.Minio.AccessKey)
	}
	if cfg.Storage.Minio.SecretKey != "env-secret" {
		t.Fatalf("SecretKey = %q, want env-secret", cfg.Storage.Minio.SecretKey)
	}
}

func TestLoad_databaseBindEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
server:
  port: "8080"
  mode: debug
database:
  host: placeholder
  port: "5432"
  user: postgres
  password: postgres
  dbname: go_template
  sslmode: disable
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime: 30
jwt:
  secret: x
  expire_hours: 72
scheduler:
  enabled: false
snowflake:
  node_id: 1
rate_limit:
  enabled: false
websocket:
  enabled: false
storage:
  driver: local
  local:
    upload_dir: ./uploads
    max_file_size: 10
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DATABASE_HOST", "db.example")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Database.Host != "db.example" {
		t.Fatalf("Host = %q, want db.example", cfg.Database.Host)
	}
}
