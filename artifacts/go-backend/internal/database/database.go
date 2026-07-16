package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a pgxpool for long-lived worker connections (prepared statements allowed).
func NewPool(ctx context.Context, connURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connURL)
	if err != nil {
		return nil, fmt.Errorf("database: parse config: %w", err)
	}

	// Direct pool is used by the long-lived workers (prepared statements OK).
	// Supabase free tier caps total DIRECT Postgres connections low (~60), and
	// cmd/server opens this pool AND the pooler pool from one process, so keep
	// this small: the workers are sequential and never need many connections.
	// Override with DB_MAX_CONNS only if you know the instance's real limit.
	maxConns := 5
	if envMax := os.Getenv("DB_MAX_CONNS"); envMax != "" {
		if val, err := strconv.Atoi(envMax); err == nil && val > 0 {
			maxConns = val
		}
	}
	cfg.MaxConns = int32(maxConns)
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: new pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database: ping failed: %w", err)
	}

	return pool, nil
}

// NewPoolerPool creates a pgxpool for the Supabase transaction pooler (Supavisor).
// Supavisor in transaction mode does NOT support prepared statements, so we use
// simple query protocol (QueryExecModeSimpleProtocol).
func NewPoolerPool(ctx context.Context, connURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connURL)
	if err != nil {
		return nil, fmt.Errorf("database: parse pooler config: %w", err)
	}

	// SimpleProtocol skips prepared statement caching — required for Supavisor.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Pooler pool serves the short-lived API handlers through Supavisor, which
	// multiplexes many client sessions onto few real Postgres connections. Even
	// so, the free-tier pooler has its own client-connection cap, so keep this
	// modest — 12 is plenty for a single-VM dashboard's request rate and leaves
	// headroom under the shared budget with the direct pool above.
	maxConns := 12
	if envMax := os.Getenv("DB_POOLER_MAX_CONNS"); envMax != "" {
		if val, err := strconv.Atoi(envMax); err == nil && val > 0 {
			maxConns = val
		}
	}
	cfg.MaxConns = int32(maxConns)
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 2 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: new pooler pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database: pooler ping failed: %w", err)
	}

	return pool, nil
}
