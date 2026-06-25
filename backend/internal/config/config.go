package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Domain        string
	AdminPassword string
	JWTSecret     string
	DBPath        string
	CaddyAdminURL string
	CaddyEmail    string
	DataDir       string
	SystemCodeDir string
	Port          int
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
}

func Load() Config {
	return Config{
		Domain:        getEnv("DOMAIN", "localhost"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin"),
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		DBPath:        getEnv("DB_PATH", "./data/tamga.db"),
		CaddyAdminURL: getEnv("CADDY_ADMIN_URL", "http://localhost:2019"),
		CaddyEmail:    getEnv("CADDY_EMAIL", "admin@example.com"),
		DataDir:       getEnv("DATA_DIR", "./data"),
		SystemCodeDir: getEnv("SYSTEM_CODE_DIR", ""),
		Port:          getEnvInt("PORT", 8080),
		ReadTimeout:   time.Second * 10,
		WriteTimeout:  time.Second * 30,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
