package config

import "os"

type Config struct {
	Port     string
	MongoURI string
	DBName   string
}

func Load() *Config {
	return &Config{
		Port:     getEnv("PORT", "8080"),
		MongoURI: getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:   getEnv("MONGO_DB_NAME", "nemesis"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
