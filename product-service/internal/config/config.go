package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Cache CacheConfig
	Queue  QueueConfig
	Worker WorkerConfig
	Log    LogConfig
	Otel   OtelConfig
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

type DatabaseConfig struct {
	Driver          string
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	AutoMigrate     bool
	MigrationsPath  string
}

func (c *DatabaseConfig) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

func (c *DatabaseConfig) MySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		c.User, c.Password, c.Host, c.Port, c.Name,
	)
}

type CacheConfig struct {
	Addr     string
	Password string
	DB       int
}

type QueueConfig struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Prefix        string
}

type WorkerConfig struct {
	NumWorkers   int
	PollInterval time.Duration
	QueueName    string
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
			Port:              8080,
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
			Enabled:      envBool("OTEL_ENABLED", true),
			ServiceName:  getEnv("OTEL_SERVICE_NAME", appName),
			Endpoint:     getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Insecure:     envBool("OTEL_INSECURE", true),
			ResourceAttrs: map[string]string{},
		},
	}
	cfg.Database = DatabaseConfig{
		Driver:          "postgres",
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            envInt("DB_PORT",5432),
		User:            getEnv("DB_USER", "postgres"),
		Password:        getEnv("DB_PASSWORD", "postgres"),
		Name:            getEnv("DB_NAME", appName),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		DSN:             getEnv("DB_DSN", "file:./data/app.db?_foreign_keys=on"),
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		AutoMigrate:     envBool("DB_AUTO_MIGRATE", false),
		MigrationsPath:  "./migrations",
	}
	cfg.Cache = CacheConfig{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       envInt("REDIS_DB", 0),
	}
	cfg.Queue = QueueConfig{
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),
		Prefix:        appName,
	}
	cfg.Worker = WorkerConfig{
		NumWorkers:   5,
		PollInterval: time.Second,
		QueueName:    "tasks",
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
