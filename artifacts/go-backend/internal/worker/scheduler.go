package worker

import (
	"context"
	"time"

	gocron "github.com/go-co-op/gocron/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/financeintel/backend/internal/config"
	"github.com/financeintel/backend/internal/worker/ai"
	"github.com/financeintel/backend/internal/worker/alert"
	"github.com/financeintel/backend/internal/worker/digest"
	"github.com/financeintel/backend/internal/worker/ipo"
	"github.com/financeintel/backend/internal/worker/market"
	"github.com/financeintel/backend/internal/worker/rss"
)

// Scheduler wraps gocron and holds references to all workers.
type Scheduler struct {
	cron         gocron.Scheduler
	RSSWorker    *rss.Worker
	AIWorker     *ai.Worker
	EnrichWorker *ai.EnrichWorker
	MktWorker    *market.Worker
	IPOWorker    *ipo.Worker
	AltWorker    *alert.Worker
	DigestWorker *digest.Worker
}

// SchedulerConfig aggregates all the workers the scheduler drives plus
// their shared configuration (intervals, etc.).
type SchedulerConfig struct {
	Pool   *pgxpool.Pool
	RSS    *rss.Worker
	AI     *ai.Worker
	Enrich *ai.EnrichWorker
	Market *market.Worker
	IPO    *ipo.Worker
	Alert  *alert.Worker
	Digest *digest.Worker
	Cfg    *config.Config
}

// Retention windows, by table. processed_items/raw_items keep a hot feed
// window; market_snapshots is high-volume (~40 symbols every few minutes)
// so it's pruned much more aggressively; alert_triggers and ai_quotas are
// tiny and kept longer for history/debugging. Previously only the first two
// were ever pruned — the other three grew unbounded.
const (
	feedDaysToKeep   = 3
	marketDaysToKeep = 3
	alertDaysToKeep  = 30
	quotaDaysToKeep  = 90
)

// pruneOldData deletes stale rows from every table that grows unbounded.
func pruneOldData(ctx context.Context, pool *pgxpool.Pool) {
	if pool == nil {
		log.Warn().Msg("scheduler: db pool is nil, skipping data pruning")
		return
	}

	prune := func(table, column string, daysToKeep int) {
		cutoff := time.Now().AddDate(0, 0, -daysToKeep)
		res, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE `+column+` < $1`, cutoff)
		if err != nil {
			log.Error().Err(err).Str("table", table).Msg("scheduler: failed to prune table")
			return
		}
		log.Info().Str("table", table).Int64("count", res.RowsAffected()).Msg("scheduler: pruned table")
	}

	prune("processed_items", "fetched_at", feedDaysToKeep)
	prune("raw_items", "fetched_at", feedDaysToKeep)
	prune("market_snapshots", "captured_at", marketDaysToKeep)
	prune("alert_triggers", "triggered_at", alertDaysToKeep)

	// Once a raw_item is processed its text is redundant with processed_items,
	// yet it otherwise lingers the full feed window — roughly doubling article
	// text storage. Drop processed rows after a day (the unprocessed inbox
	// still keeps the full window via the prune above). This is one of the
	// bigger levers on the Supabase free-tier 500MB budget.
	if res, err := pool.Exec(ctx,
		`DELETE FROM raw_items WHERE processed = TRUE AND fetched_at < $1`,
		time.Now().AddDate(0, 0, -1)); err != nil {
		log.Error().Err(err).Msg("scheduler: failed to prune processed raw_items")
	} else {
		log.Info().Int64("count", res.RowsAffected()).Msg("scheduler: pruned processed raw_items")
	}

	// ai_quotas uses a DATE column, not a timestamp — compare dates directly.
	quotaCutoff := time.Now().AddDate(0, 0, -quotaDaysToKeep).Format("2006-01-02")
	res, err := pool.Exec(ctx, `DELETE FROM ai_quotas WHERE quota_date < $1`, quotaCutoff)
	if err != nil {
		log.Error().Err(err).Msg("scheduler: failed to prune ai_quotas")
	} else {
		log.Info().Int64("count", res.RowsAffected()).Msg("scheduler: pruned ai_quotas")
	}
}

// safeRun runs a worker's Run method inside a panic guard. In the single-
// process deploy (cmd/server) every worker runs as a gocron goroutine in the
// same process as the HTTP API, so an unrecovered panic in any one worker
// would crash the API and every other worker with it. Recover, log, and let
// the next scheduled tick try again.
func safeRun(name string, fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("worker", name).Msg("scheduler: worker panicked (recovered)")
		}
	}()
	if err := fn(); err != nil {
		log.Error().Err(err).Str("worker", name).Msg("scheduler: worker error")
	}
}

// StartScheduler creates and starts the gocron scheduler with all workers.
// It returns the gocron.Scheduler so the calling binary can call .Shutdown()
// on a graceful-shutdown signal.
//
// Intervals are read from SchedulerConfig.Cfg:
//   - RSS    : every RSS_INTERVAL_MINUTES     (default 10)
//   - AI     : every AI_INTERVAL_SECONDS      (default 60)  — free pass
//   - Enrich : every ENRICH_INTERVAL_SECONDS  (default 150) — paid pass
//   - Market : every MARKET_INTERVAL_MINUTES  (default 5)
//   - IPO    : every IPO_INTERVAL_HOURS       (default 6)
//   - Alert  : every ALERT_INTERVAL_MINUTES   (default 2)
//   - Digest : once daily at DIGEST_HOUR_UTC  (default 2 UTC ≈ 7:30am IST)
func StartScheduler(ctx context.Context, sc SchedulerConfig) (gocron.Scheduler, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return nil, err
	}

	cfg := sc.Cfg

	// ── RSS Worker ─────────────────────────────────────────────────────────────
	_, err = s.NewJob(
		gocron.DurationJob(time.Duration(cfg.RSSIntervalMinutes)*time.Minute),
		gocron.NewTask(func() {
			log.Info().Msg("scheduler: rss worker starting")
			safeRun("rss_worker", func() error { return sc.RSS.Run(ctx) })
		}),
		gocron.WithName("rss_worker"),
		// Never overlap a slow cycle with the next tick — reschedule instead.
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return nil, err
	}

	// ── AI Worker (free pass) ─────────────────────────────────────────────────
	_, err = s.NewJob(
		gocron.DurationJob(time.Duration(cfg.AIIntervalSeconds)*time.Second),
		gocron.NewTask(func() {
			safeRun("ai_worker", func() error { return sc.AI.Run(ctx) })
		}),
		gocron.WithName("ai_worker"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return nil, err
	}

	// ── Enrich Worker (paid pass, gated to ai_pending rows) ────────────────────
	_, err = s.NewJob(
		gocron.DurationJob(time.Duration(cfg.EnrichIntervalSeconds)*time.Second),
		gocron.NewTask(func() {
			safeRun("enrich_worker", func() error { return sc.Enrich.Run(ctx) })
		}),
		gocron.WithName("enrich_worker"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return nil, err
	}

	// ── Market Worker ─────────────────────────────────────────────────────────
	_, err = s.NewJob(
		gocron.DurationJob(time.Duration(cfg.MarketIntervalMinutes)*time.Minute),
		gocron.NewTask(func() {
			safeRun("market_worker", func() error { return sc.Market.Run(ctx) })
		}),
		gocron.WithName("market_worker"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return nil, err
	}

	// ── IPO Worker ────────────────────────────────────────────────────────────
	_, err = s.NewJob(
		gocron.DurationJob(time.Duration(cfg.IPOIntervalHours)*time.Hour),
		gocron.NewTask(func() {
			safeRun("ipo_worker", func() error { return sc.IPO.Run(ctx) })
		}),
		gocron.WithName("ipo_worker"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return nil, err
	}

	// ── Alert Worker ──────────────────────────────────────────────────────────
	_, err = s.NewJob(
		gocron.DurationJob(time.Duration(cfg.AlertIntervalMinutes)*time.Minute),
		gocron.NewTask(func() {
			if err := sc.Alert.Run(ctx); err != nil {
				log.Error().Err(err).Msg("scheduler: alert worker error")
			}
		}),
		gocron.WithName("alert_worker"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return nil, err
	}

	// ── Digest Worker ─────────────────────────────────────────────────────────
	_, err = s.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(uint(cfg.DigestHourUTC), 0, 0))),
		gocron.NewTask(func() {
			if err := sc.Digest.Run(ctx); err != nil {
				log.Error().Err(err).Msg("scheduler: digest worker error")
			}
		}),
		gocron.WithName("digest_worker"),
	)
	if err != nil {
		return nil, err
	}

	// ── Daily Database Pruning Job ────────────────────────────────────────────
	_, err = s.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(0, 0, 0))),
		gocron.NewTask(func() {
			pruneOldData(ctx, sc.Pool)
		}),
		gocron.WithName("db_pruning"),
	)
	if err != nil {
		return nil, err
	}

	// Startup initializations
	go func() {
		// Run initial database pruning on startup
		pruneOldData(ctx, sc.Pool)
	}()

	s.Start()
	log.Info().
		Int("rss_min", cfg.RSSIntervalMinutes).
		Int("ai_sec", cfg.AIIntervalSeconds).
		Int("enrich_sec", cfg.EnrichIntervalSeconds).
		Int("market_min", cfg.MarketIntervalMinutes).
		Int("ipo_hr", cfg.IPOIntervalHours).
		Int("alert_min", cfg.AlertIntervalMinutes).
		Int("digest_hour_utc", cfg.DigestHourUTC).
		Msg("scheduler: all jobs started")
	return s, nil
}
