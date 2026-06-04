package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port        int    `mapstructure:"port"`
	DatabaseURL string `mapstructure:"database_url"`
	JWTSecret   string `mapstructure:"jwt_secret"`
}

func Load() *Config {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	v.SetDefault("port", 8080)
	v.SetDefault("database_url", "postgres://tamga:tamga@localhost:5432/tamga?sslmode=disable")
	v.SetDefault("jwt_secret", "super-secret-key-change-in-production")

	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("failed to unmarshal config: %v", err)
	}

	return &cfg
}
