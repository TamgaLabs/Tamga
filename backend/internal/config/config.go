package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Domain               string
	AdminPassword        string
	JWTSecret            string
	DBPath               string
	TraefikDynamicDir    string
	DataDir              string
	HostDataDir          string
	SystemCodeDir        string
	Port                 int
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	TraefikMetricsURL    string
	TraefikMetricsPeriod time.Duration
}

func Load() Config {
	return Config{
		Domain:            getEnv("DOMAIN", "localhost"),
		AdminPassword:     getEnv("ADMIN_PASSWORD", "admin"),
		JWTSecret:         getEnv("JWT_SECRET", "change-me-in-production"),
		DBPath:            getEnv("DB_PATH", "./data/tamga.db"),
		TraefikDynamicDir: getEnv("TRAEFIK_DYNAMIC_DIR", "/etc/traefik/dynamic"),
		DataDir:           getEnv("DATA_DIR", "./data"),
		HostDataDir:       getEnv("HOST_DATA_DIR", ""),
		SystemCodeDir:     getEnv("SYSTEM_CODE_DIR", ""),
		Port:              getEnvInt("PORT", 8080),
		ReadTimeout:       time.Second * 10,
		WriteTimeout:      time.Second * 30,
		// TraefikMetricsURL/Period (FEAT-031): the scraper's Prometheus
		// endpoint and poll interval. Defaults match TEST-010 §4's
		// confirmed in-network metrics entrypoint and the minute
		// resolution FEAT-030's schema stores samples at.
		TraefikMetricsURL:    getEnv("TRAEFIK_METRICS_URL", "http://traefik:8080/metrics"),
		TraefikMetricsPeriod: time.Duration(getEnvInt("TRAEFIK_METRICS_INTERVAL_SECONDS", 60)) * time.Second,
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
