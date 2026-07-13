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

	// CloakBrowser 环境变量绑定
	_ = v.BindEnv("cloak.base_url", "CLOAK_BASE_URL")
	_ = v.BindEnv("cloak.api_key", "CLOAK_API_KEY")

	// 服务器环境变量绑定
	_ = v.BindEnv("server.timezone", "TZ")

	// 离线任务仓库环境变量绑定
	_ = v.BindEnv("offline.store.driver", "OFFLINE_STORE_DRIVER")
	_ = v.BindEnv("offline.store.dsn", "OFFLINE_STORE_DSN", "DATABASE_URL")

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
			Port:            "8091",
			Mode:            "debug",
			Timezone:        "Asia/Shanghai",
			ShutdownTimeout: 10,
		},
		Cloak: CloakConfig{
			BaseURL: "http://localhost:3000",
		},
		Drivers: DriversConfig{
			DefaultTimeout: 30,
			Platforms:      []string{"115"},
		},
		Offline: OfflineConfig{
			Store: OfflineStoreConfig{
				Driver: "postgres",
				DSN:    "postgres://drission:drission@localhost:5432/drission_cloud_driver?sslmode=disable",
			},
		},
		RateLimit: RateLimitConfig{
			Enabled: false,
			RPS:     100,
			Burst:   200,
		},
		Modules: ModulesConfig{
			"browser": {Enabled: true},
			"driver":  {Enabled: true},
		},
	}
}
