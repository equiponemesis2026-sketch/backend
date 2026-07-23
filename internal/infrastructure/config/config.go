package config

import (
	"log"
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
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	expiry, err := time.ParseDuration(getEnv("JWT_EXPIRY", "24h"))
	if err != nil {
		expiry = 24 * time.Hour
	}

	return &Config{
		Port:      getEnv("PORT", "8080"),
		MongoURI:  getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:    getEnv("MONGO_DB_NAME", "nemesis"),
		JWTSecret: jwtSecret,
		JWTExpiry: expiry,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
