package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port      int    `mapstructure:"port"`
	DBPath    string `mapstructure:"db_path"`
	JWTSecret string `mapstructure:"jwt_secret"`
}

func Load() *Config {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	v.SetDefault("port", 8080)
	v.SetDefault("db_path", "./data/tamga.db")
	v.SetDefault("jwt_secret", "super-secret-key-change-in-production")

	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("failed to unmarshal config: %v", err)
	}

	return &cfg
}
