package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/financeintel/backend/internal/api/routes"
	"github.com/financeintel/backend/internal/config"
	"github.com/financeintel/backend/internal/database"
	"github.com/financeintel/backend/internal/embed"
	ws "github.com/financeintel/backend/internal/websocket"
	"github.com/financeintel/backend/internal/worker"
	aiworker "github.com/financeintel/backend/internal/worker/ai"
	alertworker "github.com/financeintel/backend/internal/worker/alert"
	digestworker "github.com/financeintel/backend/internal/worker/digest"
	ipoworker "github.com/financeintel/backend/internal/worker/ipo"
	marketworker "github.com/financeintel/backend/internal/worker/market"
	rssworker "github.com/financeintel/backend/internal/worker/rss"
)

func main() {
	// ── Logging ───────────────────────────────────────────────────────────────
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})

	// ── Config ────────────────────────────────────────────────────────────────
	cfg := config.Load()

	isProd := cfg.IsProduction()
	if isProd {
		gin.SetMode(gin.ReleaseMode)
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().
		Str("env", cfg.Environment).
		Int("port", cfg.Port).
		Msg("FinanceIntel Go Backend starting")

	// ── Database pools ────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Direct pool — used by workers. Prepared statements OK, long-lived connections.
	directPool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database (direct)")
	}
	defer directPool.Close()
	log.Info().Msg("database: direct pool connected")

	// Pooler pool — used by API handlers. SimpleProtocol for Supavisor compatibility.
	var poolerPool *pgxpool.Pool
	if cfg.DatabasePoolerURL == "" {
		log.Info().Msg("database: no pooler pool configured, using direct pool for API")
		poolerPool = directPool
	} else {
		var err error
		poolerPool, err = database.NewPoolerPool(ctx, cfg.DatabasePoolerURL)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to pooler database")
		}
		log.Info().Msg("database: pooler pool connected")
		defer poolerPool.Close()
	}

	// ── Background Context ────────────────────────────────────────────────────
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// ── WebSocket Hub ─────────────────────────────────────────────────────────
	hub := ws.New(bgCtx, directPool, cfg.CORSAllowedOrigins(), cfg.WSMultiInstance)
	log.Info().Msg("websocket: hub started")

	// ── Workers ───────────────────────────────────────────────────────────────
	embedClient := embed.New(cfg.EmbedSidecarURL)

	rssW := rssworker.New(directPool)
	aiW := aiworker.New(directPool, embedClient, cfg.AIBatchSize, cfg.VectorDedupThreshold, cfg.AIPendingMinFiScore, hub)
	enrichW := aiworker.NewEnrichWorker(directPool, cfg.GroqAPIKey, cfg.GeminiAPIKey, cfg.EnrichBatchSize, hub)
	marketW := marketworker.New(directPool, hub)
	ipoW := ipoworker.New(directPool)
	alertW := alertworker.New(directPool)
	digestW := digestworker.New(directPool, cfg.GeminiAPIKey)

	// ── Gin router ────────────────────────────────────────────────────────────
	r := gin.New()

	// Trust no proxies by default (nil) — only trust what's explicitly
	// configured. Without this, Gin trusts every proxy by default, which
	// means c.ClientIP() (used by both the rate limiter and the request
	// logger) honors a client-supplied X-Forwarded-For — trivially
	// spoofable — on any internet-facing deployment.
	if err := r.SetTrustedProxies(cfg.TrustedProxies()); err != nil {
		log.Fatal().Err(err).Msg("failed to set trusted proxies")
	}

	routes.Setup(r, routes.Config{
		Context:      bgCtx,
		Pool:         poolerPool,
		DirectPool:   directPool,
		WSHub:        hub,
		RSSWorker:    rssW,
		AIWorker:     aiW,
		EnrichWorker: enrichW,
		MktWorker:    marketW,
		IPOWorker:    ipoW,
		AltWorker:    alertW,
		DigestWorker: digestW,
		APIKey:       cfg.APIKey,
		CORSOrigins:  cfg.CORSAllowedOrigins(),
		Debug:        !isProd,
	})

	// ── HTTP Server ───────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Port),
		Handler: r,
		// ReadHeaderTimeout closes the slow-header (Slowloris) hole that
		// ReadTimeout alone leaves open. Kept short since all legitimate
		// clients send headers immediately.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// ── Cron Scheduler ────────────────────────────────────────────────────────

	sched, err := worker.StartScheduler(bgCtx, worker.SchedulerConfig{
		Pool:   directPool,
		RSS:    rssW,
		AI:     aiW,
		Enrich: enrichW,
		Market: marketW,
		IPO:    ipoW,
		Alert:  alertW,
		Digest: digestW,
		Cfg:    cfg,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start scheduler")
	}

	// ── Start HTTP server in background ──────────────────────────────────────
	go func() {
		log.Info().Str("addr", srv.Addr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server crashed")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutdown signal received — draining workers and connections")
	
	// Phase 1: Give workers up to 14 seconds to finish paid API calls and persist to DB
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 14*time.Second)
	defer drainCancel()
	
	drainDone := make(chan struct{})
	go func() {
		_ = sched.Shutdown() // Stops new jobs, waits for running jobs
		close(drainDone)
	}()

	select {
	case <-drainCtx.Done():
		log.Warn().Msg("worker drain timed out, forcing context cancellation")
	case <-drainDone:
		log.Info().Msg("all workers drained successfully")
	}

	// Phase 2: Cancel the global background context to kill hanging network requests
	bgCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		// Graceful drain timed out — force-close remaining connections so the
		// process doesn't hang past the shutdown window.
		log.Error().Err(err).Msg("graceful shutdown timed out — forcing close")
		_ = srv.Close()
	}
	log.Info().Msg("server stopped cleanly")
}
