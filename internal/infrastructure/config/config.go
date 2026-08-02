package config

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port                   string
	MongoURI               string
	DBName                 string
	JWTSecret              string
	JWTExpiry              time.Duration
	StripeKey              string
	StripeWebhookSecret    string
	StripePricePro         string
	StripePriceFamiliar    string
	CORSAllowedOrigins     []string
	FirebaseServiceAccount string
}

func Load() *Config {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		log.Fatal("STRIPE_SECRET_KEY is required")
	}

	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Fatal("STRIPE_WEBHOOK_SECRET is required")
	}

	expiry, err := time.ParseDuration(getEnv("JWT_EXPIRY", "24h"))
	if err != nil {
		expiry = 24 * time.Hour
	}

	return &Config{
		Port:                   getEnv("PORT", "8080"),
		MongoURI:               getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:                 getEnv("MONGO_DB_NAME", "nemesis"),
		JWTSecret:              jwtSecret,
		JWTExpiry:              expiry,
		StripeKey:              stripeKey,
		StripeWebhookSecret:    webhookSecret,
		StripePricePro:         getEnv("STRIPE_PRICE_PRO", ""),
		StripePriceFamiliar:    getEnv("STRIPE_PRICE_FAMILIAR", ""),
		CORSAllowedOrigins:     splitList(getEnv("CORS_ALLOWED_ORIGINS", "*")),
		FirebaseServiceAccount: getEnv("FIREBASE_SERVICE_ACCOUNT", ""),
	}
}

// splitList convierte una cadena separada por comas en un slice.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
