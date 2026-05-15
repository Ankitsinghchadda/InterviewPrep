package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Env         string
	DatabaseURL string
	AllowOrigin string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// JWT / sessions
	JWTSecret         []byte
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	CookieDomain      string
	CookieSecure      bool
	FrontendURL       string
	PostLoginRedirect string

	// Google Cloud / Vertex AI
	GCPProject     string
	GCPLocation    string
	AgentModel     string
	AgentEnabled   bool // false => use stub reviewer (no GCP needed)
	STTLanguage    string

	// Audio storage
	AudioLocalDir string
	MaxAudioBytes int64

	// Reference-answer TTS (Google Cloud Text-to-Speech + GCS public bucket)
	AudioBucket string // public bucket name; empty disables TTS
	TTSVoice    string // e.g., en-US-Neural2-D

	// Resume upload
	MaxResumeBytes int64

	// Admin allow-list. Emails (case-insensitive) that may create categories
	// and other admin-only resources. Set via ADMIN_EMAILS=a@x.com,b@y.com.
	AdminEmails map[string]struct{}

	// Semantic dedup thresholds (cosine similarity in [0,1]). Two thresholds:
	// WarnThreshold surfaces matches live as the user types; BlockThreshold
	// causes Create to return 409 unless the request carries ?force=true.
	DedupWarnThreshold  float64
	DedupBlockThreshold float64
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, falling back to environment")
	}

	cfg := &Config{
		Port:        getEnv("SERVER_PORT", getEnv("PORT", "8080")),
		Env:         getEnv("APP_ENV", "development"),
		DatabaseURL: buildDatabaseURL(),
		AllowOrigin: getEnv("CORS_ORIGIN", "http://localhost:5173"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),

		JWTSecret:         []byte(getEnv("JWT_SECRET", "")),
		AccessTokenTTL:    getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:   getDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		CookieDomain:      getEnv("COOKIE_DOMAIN", ""),
		CookieSecure:      getBool("COOKIE_SECURE", false),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:5173"),
		PostLoginRedirect: getEnv("POST_LOGIN_REDIRECT", "/"),

		GCPProject:    getEnv("GOOGLE_CLOUD_PROJECT", ""),
		GCPLocation:   getEnv("GOOGLE_CLOUD_LOCATION", "us-central1"),
		AgentModel:    getEnv("AGENT_MODEL", "gemini-2.5-flash"),
		AgentEnabled:  getBool("AGENT_ENABLED", false),
		STTLanguage:   getEnv("STT_LANGUAGE", "en-US"),

		AudioLocalDir:  getEnv("AUDIO_LOCAL_DIR", "./var/audio"),
		MaxAudioBytes:  getInt64("MAX_AUDIO_BYTES", 15*1024*1024),   // 15MB ~ 5min webm/opus
		MaxResumeBytes: getInt64("MAX_RESUME_BYTES", 10*1024*1024), // 10MB PDF cap

		AudioBucket: getEnv("AUDIO_BUCKET", ""),
		TTSVoice:    getEnv("TTS_VOICE", "en-US-Neural2-D"),

		DedupWarnThreshold:  getFloat("DEDUP_WARN_THRESHOLD", 0.78),
		DedupBlockThreshold: getFloat("DEDUP_BLOCK_THRESHOLD", 0.88),

		AdminEmails: parseEmailSet(getEnv("ADMIN_EMAILS", "")),
	}

	if cfg.Env == "production" {
		if len(cfg.JWTSecret) < 32 {
			log.Fatal("JWT_SECRET must be set to at least 32 bytes in production")
		}
		if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
			log.Fatal("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set in production")
		}
	} else if len(cfg.JWTSecret) == 0 {
		log.Println("WARN: JWT_SECRET is empty; using insecure dev fallback")
		cfg.JWTSecret = []byte("dev-insecure-secret-change-me-please-32b")
	}

	if cfg.AgentEnabled && cfg.GCPProject == "" {
		log.Println("WARN: AGENT_ENABLED=true but GOOGLE_CLOUD_PROJECT is empty; falling back to stub reviewer")
		cfg.AgentEnabled = false
	}

	return cfg
}

func buildDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "interviewprep")
	pass := getEnv("DB_PASSWORD", "interviewprep")
	name := getEnv("DB_NAME", "interviewprep")
	ssl := getEnv("DB_SSLMODE", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getInt64(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

// parseEmailSet splits a comma-separated env value into a lower-cased set.
// Used for the ADMIN_EMAILS allow-list so lookups are O(1) and case-insensitive.
func parseEmailSet(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, p := range strings.Split(raw, ",") {
		if e := strings.ToLower(strings.TrimSpace(p)); e != "" {
			out[e] = struct{}{}
		}
	}
	return out
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
