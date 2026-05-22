package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// InitEnv loads optional `.env` from the working directory or any parent directory
// (if present), then binds environment variables. Export vars override `.env`.
// Set ENV_FILE to load a specific path. Call once before DefaultConfig.
func InitEnv() error {
	envPath, err := findEnvFile(".env")
	if err != nil {
		return err
	}
	if envPath != "" {
		viper.SetConfigFile(envPath)
		viper.SetConfigType("env")
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("read .env: %w", err)
		}
	}
	viper.AutomaticEnv()
	return nil
}

func findEnvFile(name string) (string, error) {
	if p := os.Getenv("ENV_FILE"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("ENV_FILE %q: %w", p, err)
		}
		return p, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil
}
