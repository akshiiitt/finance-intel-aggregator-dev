DROP TABLE IF EXISTS daily_digests;

DROP INDEX IF EXISTS idx_processed_niches;
DROP INDEX IF EXISTS idx_processed_ai_pending;
DROP INDEX IF EXISTS idx_processed_embedding;

ALTER TABLE processed_items
    DROP COLUMN IF EXISTS ai_enriched,
    DROP COLUMN IF EXISTS ai_pending,
    DROP COLUMN IF EXISTS niches,
    DROP COLUMN IF EXISTS embedding;
