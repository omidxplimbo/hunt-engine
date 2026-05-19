package nuclei

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeoutSeconds = 1800
	DefaultRateLimit      = 50
	DefaultConcurrency    = 10
	DefaultBulkSize       = 25
	DefaultTemplatesDir   = "/root/nuclei-templates"
	DefaultCustomDir      = "/data/nuclei/custom"
	DefaultProfile        = "safe"
)

type Config struct {
	Timeout            time.Duration
	RateLimit          int
	Concurrency        int
	BulkSize           int
	TemplatesDir       string
	CustomTemplatesDir string
	DefaultProfile     string
	AllowAITemplates   bool
}

func LoadConfig() Config {
	return Config{
		Timeout:            durationFromEnv("NUCLEI_TIMEOUT_SECONDS", DefaultTimeoutSeconds),
		RateLimit:          intFromEnv("NUCLEI_RATE_LIMIT", DefaultRateLimit),
		Concurrency:        intFromEnv("NUCLEI_CONCURRENCY", DefaultConcurrency),
		BulkSize:           intFromEnv("NUCLEI_BULK_SIZE", DefaultBulkSize),
		TemplatesDir:       stringFromEnv("NUCLEI_TEMPLATES_DIR", DefaultTemplatesDir),
		CustomTemplatesDir: stringFromEnv("NUCLEI_CUSTOM_TEMPLATES_DIR", DefaultCustomDir),
		DefaultProfile:     NormalizeProfile(stringFromEnv("NUCLEI_DEFAULT_PROFILE", DefaultProfile)),
		AllowAITemplates:   boolFromEnv("NUCLEI_ALLOW_AI_TEMPLATES", false),
	}
}

func NormalizeProfile(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "safe", "exposure", "misconfig", "cves-light", "custom", "full":
		return v
	default:
		return DefaultProfile
	}
}

func durationFromEnv(key string, fallbackSeconds int) time.Duration {
	seconds := intFromEnv(key, fallbackSeconds)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func intFromEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("⚠️ Invalid %s=%q. Using default %d.\n", key, value, fallback)
		return fallback
	}
	return n
}

func stringFromEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func boolFromEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		log.Printf("⚠️ Invalid %s=%q. Using default %v.\n", key, value, fallback)
		return fallback
	}
}
