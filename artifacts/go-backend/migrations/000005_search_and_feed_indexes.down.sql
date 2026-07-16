-- Recreate the JSONB GIN indexes dropped in the up migration.
CREATE INDEX IF NOT EXISTS idx_processed_companies_gin ON processed_items USING gin (companies);
CREATE INDEX IF NOT EXISTS idx_processed_investors_gin ON processed_items USING gin (investors);

DROP INDEX IF EXISTS idx_processed_investors_text_trgm;
DROP INDEX IF EXISTS idx_processed_companies_text_trgm;
DROP INDEX IF EXISTS idx_processed_summary_trgm;
DROP INDEX IF EXISTS idx_processed_feed_order;

-- Recreate dropped fi_score index
CREATE INDEX IF NOT EXISTS idx_processed_fi_score ON processed_items(fi_score DESC NULLS LAST, fetched_at DESC);
