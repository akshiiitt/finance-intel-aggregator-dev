-- raw_items: inbox for all RSS fetched articles
CREATE TABLE IF NOT EXISTS raw_items (
    id              SERIAL PRIMARY KEY,
    source          TEXT NOT NULL,
    source_type     TEXT NOT NULL DEFAULT 'rss',
    url             TEXT UNIQUE,
    title           TEXT NOT NULL,
    snippet         TEXT,
    author          TEXT,
    published_at    TIMESTAMPTZ,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    content_hash    TEXT UNIQUE,
    processed       BOOLEAN NOT NULL DEFAULT FALSE,
    processing_error TEXT
);

-- processed_items: AI-enriched articles with FI scores
CREATE TABLE IF NOT EXISTS processed_items (
    id              SERIAL PRIMARY KEY,
    raw_item_id     INTEGER,
    title           TEXT NOT NULL,
    summary         TEXT,
    key_points      TEXT,
    source_url      TEXT NOT NULL,
    source          TEXT NOT NULL,
    source_type     TEXT,
    region          TEXT,
    category        TEXT,
    sentiment       TEXT,
    sentiment_score NUMERIC,
    relevance_score NUMERIC,
    fi_score        NUMERIC,
    companies       TEXT,
    investors       TEXT,
    amount          NUMERIC,
    currency        TEXT,
    round_type      TEXT,
    valuation       NUMERIC,
    coverage_count  INTEGER DEFAULT 1,
    also_sources    TEXT,
    ai_model_used   TEXT,
    published_at    TIMESTAMPTZ,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- market_snapshots: time-series price data for all symbols
CREATE TABLE IF NOT EXISTS market_snapshots (
    id          SERIAL PRIMARY KEY,
    symbol      TEXT NOT NULL,
    name        TEXT,
    exchange    TEXT NOT NULL,
    price       NUMERIC NOT NULL,
    change_pct  NUMERIC,
    change_abs  NUMERIC,
    prev_close  NUMERIC,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ipo_calendar: IPO tracking (NSE/BSE)
CREATE TABLE IF NOT EXISTS ipo_calendar (
    id              SERIAL PRIMARY KEY,
    company_name    TEXT NOT NULL UNIQUE,
    exchange        TEXT,
    price_band_low  NUMERIC,
    price_band_high NUMERIC,
    lot_size        INTEGER,
    open_date       DATE,
    close_date      DATE,
    listing_date    DATE,
    issue_size_cr   NUMERIC,
    gmp             NUMERIC,
    subscription_x  NUMERIC,
    status          TEXT,
    sector          TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- alerts: user-defined notification rules
CREATE TABLE IF NOT EXISTS alerts (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,
    conditions      JSONB NOT NULL,
    is_active       BOOLEAN DEFAULT TRUE,
    last_triggered  TIMESTAMPTZ,
    trigger_count   INTEGER DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- alert_triggers: history of matched alerts
CREATE TABLE IF NOT EXISTS alert_triggers (
    id          SERIAL PRIMARY KEY,
    alert_id    INTEGER NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    article_id  INTEGER NOT NULL,
    title       TEXT NOT NULL,
    source      TEXT,
    category    TEXT,
    fi_score    NUMERIC,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(alert_id, article_id)
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_processed_fi_score   ON processed_items(fi_score DESC NULLS LAST, fetched_at DESC);
CREATE INDEX IF NOT EXISTS idx_processed_category   ON processed_items(category) WHERE category IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_processed_region     ON processed_items(region);
CREATE INDEX IF NOT EXISTS idx_processed_fetched    ON processed_items(fetched_at DESC);
CREATE INDEX IF NOT EXISTS idx_processed_amount     ON processed_items(amount DESC NULLS LAST) WHERE amount IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_processed_source     ON processed_items(source);
CREATE INDEX IF NOT EXISTS idx_processed_source_url ON processed_items(source_url);
CREATE INDEX IF NOT EXISTS idx_raw_unprocessed      ON raw_items(fetched_at) WHERE processed = FALSE;
CREATE INDEX IF NOT EXISTS idx_market_symbol_time   ON market_snapshots(symbol, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_triggers_time  ON alert_triggers(triggered_at DESC);

-- Trigram index for fast ILIKE title searching
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_processed_title_trgm ON processed_items USING gin (title gin_trgm_ops);
