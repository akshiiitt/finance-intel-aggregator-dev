-- Adds the free semantic layer (embeddings + niches) and splits AI processing
-- into a free pass (Process worker, already ran) and a paid pass (Enrich
-- worker) gated by ai_pending. See docs on the two-worker AI pipeline.

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE processed_items
    ADD COLUMN IF NOT EXISTS embedding    vector(384),
    ADD COLUMN IF NOT EXISTS niches       TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS ai_pending   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ai_enriched  BOOLEAN NOT NULL DEFAULT FALSE;

-- Approximate nearest-neighbor index for cosine-distance dedup + semantic
-- search. Only ever queried against the hot retention window (a few days),
-- so this stays small and fast even on a free-tier Postgres instance.
CREATE INDEX IF NOT EXISTS idx_processed_embedding
    ON processed_items USING hnsw (embedding vector_cosine_ops);

-- Lets the Enrich worker's "pick the next batch to spend AI budget on" query
-- use an index instead of a sequential scan.
CREATE INDEX IF NOT EXISTS idx_processed_ai_pending
    ON processed_items (fi_score DESC) WHERE ai_pending = TRUE AND ai_enriched = FALSE;

CREATE INDEX IF NOT EXISTS idx_processed_niches
    ON processed_items USING gin (niches);

-- One row per day: the real AI-generated morning briefing, built once from
-- the day's top-scored stories (see internal/worker/digest).
CREATE TABLE IF NOT EXISTS daily_digests (
    id            SERIAL PRIMARY KEY,
    digest_date   DATE UNIQUE NOT NULL,
    content       TEXT NOT NULL,
    top_story_ids INTEGER[] NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
