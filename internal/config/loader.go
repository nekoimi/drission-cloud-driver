package config

import (
	"strings"

	"github.com/spf13/viper"
)

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(configPath)

	// 环境变量绑定
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 数据库环境变量绑定
	_ = v.BindEnv("database.host", "DATABASE_HOST")
	_ = v.BindEnv("database.port", "DATABASE_PORT")
	_ = v.BindEnv("database.user", "DATABASE_USER")
	_ = v.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = v.BindEnv("database.dbname", "DATABASE_NAME")
	_ = v.BindEnv("jwt.secret", "JWT_SECRET")
	_ = v.BindEnv("server.timezone", "TZ")
	_ = v.BindEnv("snowflake.node_id", "SNOWFLAKE_NODE_ID")

	_ = v.BindEnv("storage.minio.access_key", "MINIO_ACCESS_KEY")
	_ = v.BindEnv("storage.minio.secret_key", "MINIO_SECRET_KEY")
	_ = v.BindEnv("storage.minio.endpoint", "MINIO_ENDPOINT")
	_ = v.BindEnv("storage.minio.public_url", "MINIO_PUBLIC_URL")
	_ = v.BindEnv("storage.minio.bucket", "MINIO_BUCKET")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            "8080",
			Mode:            "debug",
			Timezone:        "Asia/Shanghai",
			ShutdownTimeout: 10,
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            "5432",
			User:            "postgres",
			Password:        "postgres",
			DBName:          "go_template",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30,
		},
		JWT: JWTConfig{
			Secret:      "change-me-in-production",
			ExpireHours: 72,
		},
		Scheduler: SchedulerConfig{
			Enabled:  false,
			Timezone: "Asia/Shanghai",
		},
		Snowflake: SnowflakeConfig{
			NodeID: 1,
		},
		RateLimit: RateLimitConfig{
			Enabled: false,
			RPS:     100,
			Burst:   200,
		},
		Websocket: WebsocketConfig{
			Enabled:         false,
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			PingPeriod:      60 * 1e9,  // 60s
			WriteWait:       10 * 1e9,  // 10s
			ReadWait:        60 * 1e9,  // 60s
			MaxMessageSize:  5120,
		},
		Storage: StorageConfig{
			Driver:  "local",
			BaseURL: "http://localhost:8080/uploads",
			Local: LocalConfig{
				UploadDir:   "./uploads",
				MaxFileSize: 10,
			},
			Minio: MinioConfig{
				Endpoint:  "localhost:9000",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin",
				Bucket:    "go-template",
				UseSSL:    false,
				PublicURL: "http://localhost:9000",
			},
		},
	}
}
