package config

import (
	"os"
	"time"
)

type Config struct {
	Env            string
	HTTPAddr       string
	DatabaseURL    string
	JWTIssuer      string
	JWTPrivateKey  string
	AccessTokenTTL time.Duration
}

func Load() Config {
	ttl, err := time.ParseDuration(get("ACCESS_TOKEN_TTL", "15m"))
	if err != nil || ttl < time.Minute || ttl > time.Hour {
		ttl = 15 * time.Minute
	}
	return Config{
		Env:            environment(),
		HTTPAddr:       get("HTTP_ADDR", ":8080"),
		DatabaseURL:    get("DATABASE_URL", "postgres://inspections:inspections_dev_password@localhost:5432/inspections?sslmode=disable"),
		JWTIssuer:      get("JWT_ISSUER", "inspections-api"),
		JWTPrivateKey:  get("JWT_PRIVATE_KEY", ""),
		AccessTokenTTL: ttl,
	}
}

func get(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

// Only an explicit development value enables development credentials and OTPs.
func environment() string {
	if os.Getenv("APP_ENV") == "development" {
		return "development"
	}
	return "production"
}
