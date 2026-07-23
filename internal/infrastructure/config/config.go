package config

import (
	"os"
	"time"
)

type Config struct {
	Port      string
	MongoURI  string
	DBName    string
	JWTSecret string
	JWTExpiry time.Duration
}

func Load() *Config {
	expiry, err := time.ParseDuration(getEnv("JWT_EXPIRY", "24h"))
	if err != nil {
		expiry = 24 * time.Hour
	}

	return &Config{
		Port:      getEnv("PORT", "8080"),
		MongoURI:  getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:    getEnv("MONGO_DB_NAME", "nemesis"),
		JWTSecret: getEnv("JWT_SECRET", "super-secret-key"),
		JWTExpiry: expiry,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
