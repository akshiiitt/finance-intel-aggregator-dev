package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration values loaded from environment variables or .env file.
type Config struct {
	Port              int    `mapstructure:"PORT"`
	Environment       string `mapstructure:"ENVIRONMENT"`
	DatabaseURL       string `mapstructure:"DATABASE_URL"`
	DatabasePoolerURL string `mapstructure:"DATABASE_POOLER_URL"`
	GroqAPIKey        string `mapstructure:"GROQ_API_KEY"`
	GeminiAPIKey      string `mapstructure:"GEMINI_API_KEY"`

	// APIKey guards every mutating route and worker trigger (see
	// internal/api/middleware/auth.go). Required whenever Environment is
	// "production" — Load() refuses to start otherwise. This is a
	// single-tenant personal tool, so one shared secret is the right amount
	// of auth, not full user accounts.
	APIKey string `mapstructure:"API_KEY"`

	// CORSAllowedOriginsRaw is the comma-separated env value; use
	// CORSAllowedOrigins() for the parsed slice.
	CORSAllowedOriginsRaw string `mapstructure:"CORS_ALLOWED_ORIGINS"`

	// TrustedProxiesRaw is the comma-separated env value; use
	// TrustedProxies() for the parsed slice. Passed straight to Gin's
	// SetTrustedProxies — leave empty (the default) to trust no proxies,
	// which is correct unless this actually sits behind a reverse proxy /
	// load balancer that sets X-Forwarded-For.
	TrustedProxiesRaw string `mapstructure:"TRUSTED_PROXIES"`

	RSSIntervalMinutes    int `mapstructure:"RSS_INTERVAL_MINUTES"`
	AIIntervalSeconds     int `mapstructure:"AI_INTERVAL_SECONDS"`
	EnrichIntervalSeconds int `mapstructure:"ENRICH_INTERVAL_SECONDS"`
	MarketIntervalMinutes int `mapstructure:"MARKET_INTERVAL_MINUTES"`
	IPOIntervalHours      int `mapstructure:"IPO_INTERVAL_HOURS"`
	AlertIntervalMinutes  int `mapstructure:"ALERT_INTERVAL_MINUTES"`
	DigestHourUTC         int `mapstructure:"DIGEST_HOUR_UTC"`

	// AIBatchSize is how many raw_items the free Process worker classifies,
	// embeds, and scores per cycle. EnrichBatchSize is how many ai_pending
	// rows the paid Enrich worker spends budget on per cycle — kept
	// separate because these two workers run on different schedules and
	// have very different cost profiles.
	AIBatchSize     int `mapstructure:"AI_BATCH_SIZE"`
	EnrichBatchSize int `mapstructure:"ENRICH_BATCH_SIZE"`

	// EmbedSidecarURL points at the local fastembed sidecar (see
	// artifacts/embed-sidecar) — free, unlimited, no external API.
	EmbedSidecarURL string `mapstructure:"EMBED_SIDECAR_URL"`

	// VectorDedupThreshold is a cosine SIMILARITY (not distance) — two
	// articles at or above this are treated as the same story. Calibrate
	// against real near-duplicate/non-duplicate pairs before trusting the
	// default; see internal/worker/ai/worker.go's checkDuplicateVector.
	VectorDedupThreshold float64 `mapstructure:"VECTOR_DEDUP_THRESHOLD"`

	// AIPendingMinFiScore is the FIScore a non-deal article needs before the
	// free Process worker flags it for a paid AI summary. Deal-type articles
	// (funding/ipo/mergers) are always flagged regardless of this value.
	AIPendingMinFiScore float64 `mapstructure:"AI_PENDING_MIN_FI_SCORE"`

	// WSMultiInstance enables cross-process WebSocket fan-out via Postgres
	// LISTEN/NOTIFY. Leave false (the default) for the single-VM deployment
	// where cmd/server runs the API and all workers in one process — there,
	// broadcasts are delivered directly in-memory, which avoids a per-message
	// DB round-trip and frees the dedicated LISTEN connection. Only set true
	// if you run cmd/worker as a separate process that must push events to a
	// separately-running API server.
	WSMultiInstance bool `mapstructure:"WS_MULTI_INSTANCE"`
}

// IsProduction reports whether this is a production deploy. Anything other
// than the literal string "development" counts as production — that's the
// fail-closed direction: an unset or misspelled ENVIRONMENT locks pprof and
// debug logging down rather than opening them up.
func (c *Config) IsProduction() bool {
	return c.Environment != "development"
}

// CORSAllowedOrigins parses the comma-separated CORS_ALLOWED_ORIGINS value.
func (c *Config) CORSAllowedOrigins() []string {
	return splitCSV(c.CORSAllowedOriginsRaw)
}

// TrustedProxies parses the comma-separated TRUSTED_PROXIES value. An empty
// result means "trust no proxies," which is what gin.SetTrustedProxies(nil)
// expects and is the correct default for a server not sitting behind a
// reverse proxy.
func (c *Config) TrustedProxies() []string {
	return splitCSV(c.TrustedProxiesRaw)
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Load reads config from .env file and environment variables.
// Environment variables always override .env file values.
func Load() *Config {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	viper.SetDefault("PORT", 8080)
	// Fail-closed default: anything not explicitly "development" is treated
	// as production (see IsProduction). Previously this defaulted to
	// "development," so an unset ENVIRONMENT in a real deploy silently left
	// pprof registered and logs at debug level.
	viper.SetDefault("ENVIRONMENT", "production")
	viper.SetDefault("RSS_INTERVAL_MINUTES", 10)
	viper.SetDefault("AI_INTERVAL_SECONDS", 60)
	viper.SetDefault("ENRICH_INTERVAL_SECONDS", 150)
	viper.SetDefault("MARKET_INTERVAL_MINUTES", 5)
	viper.SetDefault("IPO_INTERVAL_HOURS", 6)
	viper.SetDefault("ALERT_INTERVAL_MINUTES", 2)
	viper.SetDefault("DIGEST_HOUR_UTC", 2) // ~7:30am IST
	viper.SetDefault("AI_BATCH_SIZE", 30)
	viper.SetDefault("ENRICH_BATCH_SIZE", 30)
	viper.SetDefault("EMBED_SIDECAR_URL", "http://127.0.0.1:8900")
	viper.SetDefault("VECTOR_DEDUP_THRESHOLD", 0.88)
	viper.SetDefault("AI_PENDING_MIN_FI_SCORE", 70)
	viper.SetDefault("WS_MULTI_INSTANCE", false)
	viper.SetDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:8080")
	viper.SetDefault("TRUSTED_PROXIES", "")

	// viper.Unmarshal only pulls a bare environment variable (no .env file,
	// no config-file entry) into the struct if the key was already
	// registered — AutomaticEnv() alone does not add unseen keys to
	// viper's key set. Without these empty-string defaults, every one of
	// these came through as "" in any env-vars-only deployment (e.g. this
	// project's own docker-compose.yml), which is exactly the silent
	// failure mode the fail-closed API_KEY/DATABASE_URL checks below are
	// supposed to catch — so the keys must be registered even though the
	// "real" values only ever come from the environment, never a default.
	viper.SetDefault("DATABASE_URL", "")
	viper.SetDefault("DATABASE_POOLER_URL", "")
	viper.SetDefault("GROQ_API_KEY", "")
	viper.SetDefault("GEMINI_API_KEY", "")
	viper.SetDefault("API_KEY", "")

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("config: failed to unmarshal: %v", err)
	}

	// Validate numeric intervals and batch sizes are strictly positive to prevent scheduler panics.
	if cfg.RSSIntervalMinutes <= 0 {
		log.Fatal("config: RSS_INTERVAL_MINUTES must be greater than 0")
	}
	if cfg.AIIntervalSeconds <= 0 {
		log.Fatal("config: AI_INTERVAL_SECONDS must be greater than 0")
	}
	if cfg.EnrichIntervalSeconds <= 0 {
		log.Fatal("config: ENRICH_INTERVAL_SECONDS must be greater than 0")
	}
	if cfg.MarketIntervalMinutes <= 0 {
		log.Fatal("config: MARKET_INTERVAL_MINUTES must be greater than 0")
	}
	if cfg.IPOIntervalHours <= 0 {
		log.Fatal("config: IPO_INTERVAL_HOURS must be greater than 0")
	}
	if cfg.AlertIntervalMinutes <= 0 {
		log.Fatal("config: ALERT_INTERVAL_MINUTES must be greater than 0")
	}
	if cfg.AIBatchSize <= 0 {
		log.Fatal("config: AI_BATCH_SIZE must be greater than 0")
	}
	if cfg.EnrichBatchSize <= 0 {
		log.Fatal("config: ENRICH_BATCH_SIZE must be greater than 0")
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("config: DATABASE_URL is required")
	}

	// If no pooler URL is set, fall back to the direct URL.
	if cfg.DatabasePoolerURL == "" {
		cfg.DatabasePoolerURL = cfg.DatabaseURL
	}

	if cfg.GroqAPIKey == "" && cfg.GeminiAPIKey == "" {
		log.Println("INFO: config: neither GROQ_API_KEY nor GEMINI_API_KEY is set — the feed still works fully on the free keyword+embedding pipeline; only the Enrich worker's paid summaries/entity extraction will be skipped.")
	}

	if cfg.IsProduction() && cfg.APIKey == "" {
		log.Fatal("config: API_KEY is required when ENVIRONMENT is not \"development\" — set one (any long random string) to protect mutating routes and worker triggers. This is a fail-closed check: an unset ENVIRONMENT is treated as production, not as permission to run without auth.")
	}

	// In production the default CORS origins are localhost-only, which would
	// silently reject every credentialed request from the real (e.g. Vercel)
	// dashboard — a browser-side outage that never shows up in server logs.
	// Fail closed: force the operator to set the real origin(s) explicitly.
	if cfg.IsProduction() {
		origins := cfg.CORSAllowedOrigins()
		if len(origins) == 0 {
			log.Fatal("config: CORS_ALLOWED_ORIGINS is empty in production — set it to the exact dashboard origin(s), e.g. https://your-app.vercel.app (comma-separated, no wildcards). Without it every credentialed browser request is rejected.")
		}
		for _, o := range origins {
			if strings.Contains(o, "localhost") || strings.Contains(o, "127.0.0.1") {
				log.Fatal("config: CORS_ALLOWED_ORIGINS still contains a localhost origin in production — set it to the exact dashboard origin(s), e.g. https://your-app.vercel.app (comma-separated, no wildcards). This is a fail-closed check: the localhost default would reject every real browser request.")
			}
		}
	}
	if !cfg.IsProduction() && cfg.APIKey == "" {
		log.Println("WARNING: config: API_KEY is not set — mutating routes and worker triggers are UNPROTECTED. This is only acceptable for local development.")
	}

	return &cfg
}
