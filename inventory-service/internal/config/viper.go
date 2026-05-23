package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// InitEnv loads optional `.env` from the process working directory (if present),
// then binds environment variables. Export vars override `.env`. Call once before DefaultConfig.
func InitEnv() error {
	if _, err := os.Stat(".env"); err == nil {
		viper.SetConfigFile(".env")
		viper.SetConfigType("env")
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("read .env: %w", err)
		}
	}
	viper.AutomaticEnv()
	return nil
}
