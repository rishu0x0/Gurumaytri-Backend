package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	DatabaseURL            string
	JWTSecret              string
	JWTAccessExpiry        time.Duration
	Port                   string
	MSG91AuthToken         string
	MSG91WidgetID          string
	Env                    string
	SupabaseURL            string
	SupabaseServiceRoleKey string
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL:    requireEnv("DATABASE_URL"),
		JWTSecret:      requireEnv("JWT_SECRET"),
		Port:           getEnv("PORT", "8080"),
		MSG91AuthToken: requireEnv("MSG91_AUTH_TOKEN"),
		MSG91WidgetID:  requireEnv("MSG91_WIDGET_ID"),
		Env:                    getEnv("ENV", "development"),
		SupabaseURL:            requireEnv("SUPABASE_URL"),
		SupabaseServiceRoleKey: requireEnv("SUPABASE_SERVICE_ROLE_KEY"),
	}

	expiryStr := getEnv("JWT_ACCESS_EXPIRY", "24h")
	d, err := time.ParseDuration(expiryStr)
	if err != nil {
		log.Fatalf("invalid JWT_ACCESS_EXPIRY %q: %v", expiryStr, err)
	}
	cfg.JWTAccessExpiry = d
	return cfg
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
