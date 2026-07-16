// cmd/worker/main.go
//
// Standalone worker binary — runs all background workers (RSS, AI, Enrich,
// market, IPO, alerts, digest) without the HTTP API server. Deploy this
// separately from cmd/server if you want to scale workers independently
// from the API tier.
//
// Usage:
//
//	DATABASE_URL=... GROQ_API_KEY=... ./worker
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/financeintel/backend/internal/config"
	"github.com/financeintel/backend/internal/database"
	"github.com/financeintel/backend/internal/embed"
	"github.com/financeintel/backend/internal/fxrates"
	"github.com/financeintel/backend/internal/worker"
	aiworker "github.com/financeintel/backend/internal/worker/ai"
	alertworker "github.com/financeintel/backend/internal/worker/alert"
	digestworker "github.com/financeintel/backend/internal/worker/digest"
	ipoworker "github.com/financeintel/backend/internal/worker/ipo"
	marketworker "github.com/financeintel/backend/internal/worker/market"
	rssworker "github.com/financeintel/backend/internal/worker/rss"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})

	cfg := config.Load()

	if cfg.IsProduction() {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Str("env", cfg.Environment).Msg("FinanceIntel worker starting")

	// ── Database (workers use direct pool, no Supavisor) ─────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	log.Info().Msg("database: direct pool connected")

	// ── Background Context ────────────────────────────────────────────────────
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// ── FX Rate Poller ────────────────────────────────────────────────────────
	fxrates.Global.StartBackgroundRefresh(bgCtx, pool, 5*time.Minute)

	// ── Workers (no WebSocket hub — worker binary has no clients) ─────────────
	embedClient := embed.New(cfg.EmbedSidecarURL)

	rssW := rssworker.New(pool)
	aiW := aiworker.New(pool, embedClient, cfg.AIBatchSize, cfg.VectorDedupThreshold, cfg.AIPendingMinFiScore, nil)
	enrichW := aiworker.NewEnrichWorker(pool, cfg.GroqAPIKey, cfg.GeminiAPIKey, cfg.EnrichBatchSize, nil)
	marketW := marketworker.New(pool, nil)
	ipoW := ipoworker.New(pool)
	alertW := alertworker.New(pool)
	digestW := digestworker.New(pool, cfg.GeminiAPIKey)

	// ── Scheduler ─────────────────────────────────────────────────────────────
	sched, err := worker.StartScheduler(bgCtx, worker.SchedulerConfig{
		Pool:   pool,
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


	log.Info().Msg("all workers started — waiting for shutdown signal")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutdown signal received — draining workers")
	
	// Two-Phase Shutdown:
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
	log.Info().Msg("worker binary stopped cleanly")
}
