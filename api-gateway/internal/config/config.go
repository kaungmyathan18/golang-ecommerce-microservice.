package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Cache    CacheConfig
	Gateway  GatewayConfig
	Log      LogConfig
	Otel     OtelConfig
}

type GatewayConfig struct {
	JWTSecret           string
	ProductServiceURL   string
	OrderServiceURL     string
	InventoryServiceURL string
	PaymentServiceURL   string
	AuthServiceURL      string
	RateLimitRPM        int
}

type AppConfig struct {
	Name        string
	Version     string
	Environment string
}

type ServerConfig struct {
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	CORS              CORSConfig
}

type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

type CacheConfig struct {
	Addr     string
	Password string
	DB       int
}

type LogConfig struct {
	Level  string
	Format string
}

type OtelConfig struct {
	Enabled       bool
	ServiceName   string
	Endpoint      string
	Insecure      bool
	ResourceAttrs map[string]string
}

func DefaultConfig(appName string) *Config {
	cfg := &Config{
		App: AppConfig{
			Name:        appName,
			Version:     "0.1.0",
			Environment: getEnv("APP_ENV", "development"),
		},
		Server: ServerConfig{
			Port:              envInt("SERVER_PORT", 8080),
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			CORS: CORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
				ExposedHeaders:   []string{"X-Request-ID"},
				AllowCredentials: false,
				MaxAge:           300,
			},
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "console"),
		},
		Otel: OtelConfig{
			Enabled:       envBool("OTEL_ENABLED", false),
			ServiceName:   getEnv("OTEL_SERVICE_NAME", appName),
			Endpoint:      getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Insecure:      envBool("OTEL_INSECURE", true),
			ResourceAttrs: map[string]string{},
		},
	}
	cfg.Cache = CacheConfig{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       envInt("REDIS_DB", 0),
	}
	cfg.Gateway = GatewayConfig{
		JWTSecret:           getEnv("JWT_SECRET", "dev-secret-change-me"),
		ProductServiceURL:   getEnv("PRODUCT_SERVICE_URL", "http://localhost:8081"),
		OrderServiceURL:     getEnv("ORDER_SERVICE_URL", "http://localhost:8082"),
		InventoryServiceURL: getEnv("INVENTORY_SERVICE_URL", "http://localhost:8084"),
		PaymentServiceURL:   getEnv("PAYMENT_SERVICE_URL", "http://localhost:8085"),
		AuthServiceURL:      getEnv("AUTH_SERVICE_URL", "http://localhost:8086"),
		RateLimitRPM:        envInt("RATE_LIMIT_RPM", 60),
	}
	return cfg
}

func getEnv(k, def string) string {
	if !viper.IsSet(k) {
		return def
	}
	return viper.GetString(k)
}

func envBool(k string, def bool) bool {
	if !viper.IsSet(k) {
		return def
	}
	return viper.GetBool(k)
}

func envInt(k string, def int) int {
	if !viper.IsSet(k) {
		return def
	}
	v := viper.GetInt(k)
	if v == 0 && viper.GetString(k) == "" {
		return def
	}
	return v
}
