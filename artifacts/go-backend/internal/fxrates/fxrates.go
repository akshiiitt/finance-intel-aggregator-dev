// Package fxrates maintains a live USD/INR exchange rate cache populated from
// the market_snapshots table (USDINR=X symbol tracked by the market worker).
//
// It is the single source of truth for FX conversion across all handlers and
// workers — replacing every hardcoded "* 85" in the original code.
//
// Usage:
//
//	fxrates.Global.GetUSDINR()        // → e.g. 84.52
//	fxrates.Global.USDCroreFactor()   // → e.g. 8.452  (= usd_inr / 10)
package fxrates

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// DefaultUSDINR is the conservative fallback used before the first successful
// DB read. 84.0 is approximately the mid-market rate at launch; it is only
// used if the market worker has not run yet.
const DefaultUSDINR = 84.0

// Cache is a goroutine-safe in-process FX rate store.
// Only one instance is needed per process — use the Global singleton.
type Cache struct {
	mu     sync.RWMutex
	usdINR float64 // 0 = not yet populated
}

// Global is the process-wide FX rate cache.
var Global = &Cache{}

// GetUSDINR returns the current USD/INR spot rate (e.g. 84.52).
// Falls back to DefaultUSDINR if the cache has never been refreshed.
func (c *Cache) GetUSDINR() float64 {
	c.mu.RLock()
	r := c.usdINR
	c.mu.RUnlock()
	if r <= 0 {
		return DefaultUSDINR
	}
	return r
}

// USDCroreFactor returns the multiplier for converting raw USD amounts to
// Indian crore:
//
//	$1 M USD  ×  rate  =  rate × 10⁶ INR  =  (rate / 10) crore
//
// Example: rate = 84.52  →  factor = 8.452
// So  $10 M  →  10 × 8.452  =  84.52 crore  ✓
func (c *Cache) USDCroreFactor() float64 {
	return c.GetUSDINR() / 10.0
}

// set atomically replaces the cached rate.
func (c *Cache) set(rate float64) {
	c.mu.Lock()
	c.usdINR = rate
	c.mu.Unlock()
}

// Refresh reads the most recent USDINR=X snapshot from the DB.
// It is called by the market worker after every successful fetch cycle,
// and once at application startup.
// A failed refresh is a no-op — the previous cached value is preserved.
func (c *Cache) Refresh(ctx context.Context, pool *pgxpool.Pool) {
	var rate float64
	err := pool.QueryRow(ctx, `
		SELECT price::float8
		FROM market_snapshots
		WHERE symbol = 'USDINR=X'
		ORDER BY captured_at DESC
		LIMIT 1
	`).Scan(&rate)
	if err != nil || rate <= 0 {
		log.Debug().Err(err).Msg("fxrates: USDINR refresh skipped (no data yet)")
		return
	}
	c.set(rate)
	log.Debug().Float64("usdINR", rate).Msg("fxrates: USDINR updated from market snapshot")
}

// StartBackgroundRefresh launches a goroutine that refreshes the rate every
// interval from the DB. Call this once at startup after the DB pool is ready.
// Use a conservative interval (5–10 minutes) — the market worker already
// writes fresh data every MARKET_INTERVAL_MINUTES.
func (c *Cache) StartBackgroundRefresh(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	go func() {
		// Initial load
		initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		c.Refresh(initCtx, pool)
		cancel()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refreshCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				c.Refresh(refreshCtx, pool)
				cancel()
			case <-ctx.Done():
				return
			}
		}
	}()
}

