package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                     string
	MongoURI                 string
	DBName                   string
	JWTSecret                string
	JWTExpiry                time.Duration
	StripeKey                string
	StripeWebhookSecret      string
	StripePricePro           string
	StripePriceFamiliar      string
	CORSAllowedOrigins       []string
	FirebaseServiceAccount   string
	AudioMaxChunkBytes       int
	StressCriticalThreshold  float64
	StressEmotionalThreshold float64
	AIWorkerCount            int
	AIAnalyzerURL            string
	AIAnalyzerTimeout        time.Duration
	DistressConfirmChunks    int
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
		Port:                     getEnv("PORT", "8080"),
		MongoURI:                 getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:                   getEnv("MONGO_DB_NAME", "nemesis"),
		JWTSecret:                jwtSecret,
		JWTExpiry:                expiry,
		StripeKey:                stripeKey,
		StripeWebhookSecret:      webhookSecret,
		StripePricePro:           getEnv("STRIPE_PRICE_PRO", ""),
		StripePriceFamiliar:      getEnv("STRIPE_PRICE_FAMILIAR", ""),
		CORSAllowedOrigins:       splitList(getEnv("CORS_ALLOWED_ORIGINS", "*")),
		FirebaseServiceAccount:   getEnv("FIREBASE_SERVICE_ACCOUNT", ""),
		AudioMaxChunkBytes:       getEnvInt("AUDIO_MAX_CHUNK_BYTES", 1<<20),
		StressCriticalThreshold:  getEnvFloat("STRESS_CRITICAL_THRESHOLD", 0.85),
		StressEmotionalThreshold: getEnvFloat("STRESS_EMOTIONAL_THRESHOLD", 0.6),
		AIWorkerCount:            getEnvInt("AI_WORKER_COUNT", 4),
		AIAnalyzerURL:            getEnv("AI_ANALYZER_URL", "http://localhost:8000"),
		AIAnalyzerTimeout:        getEnvDuration("AI_ANALYZER_TIMEOUT", 10*time.Second),
		DistressConfirmChunks:    getEnvInt("DISTRESS_CONFIRM_CHUNKS", 3),
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

// getEnvInt parsea una variable de entorno entera con fallback.
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// getEnvFloat parsea una variable de entorno flotante con fallback.
func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

// getEnvDuration parsea una variable de entorno de duración con fallback.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
