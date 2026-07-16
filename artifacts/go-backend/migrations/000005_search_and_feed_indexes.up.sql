-- Align indexes with the queries the API and workers actually run, and stop
-- paying storage/write cost for indexes nothing uses. Matters on the free-tier
-- 500MB budget where every index and every seq scan is felt.

-- 1. Feed ordering. GetFeed's default sort is
--      ORDER BY fi_score DESC NULLS LAST, published_at DESC NULLS LAST
--    but the existing idx_processed_fi_score tiebreaks on fetched_at, so ties
--    (very common — many rows share an fi_score) forced a re-sort and made
--    deep OFFSET pagination walk extra rows. This index matches the ORDER BY
--    exactly so the planner can read straight from it.
CREATE INDEX IF NOT EXISTS idx_processed_feed_order
    ON processed_items (fi_score DESC NULLS LAST, published_at DESC NULLS LAST);

-- 2. Full-text-ish search. GetSearch / entity profile use leading-wildcard
--    ILIKE ('%q%') on title, summary, companies::text and investors::text.
--    pg_trgm GIN indexes accelerate exactly this pattern. title already has
--    one (000001); add the other three. The companies/investors indexes are
--    on the ::text form the handlers actually query.
CREATE INDEX IF NOT EXISTS idx_processed_summary_trgm
    ON processed_items USING gin (summary gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_processed_companies_text_trgm
    ON processed_items USING gin ((companies::text) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_processed_investors_text_trgm
    ON processed_items USING gin ((investors::text) gin_trgm_ops);

-- 3. Drop dead weight. The JSONB GIN indexes added in 000004 only serve
--    containment (@>) queries, but no handler uses @> — they all use
--    ::text ILIKE (now covered by the trigram indexes above). Reclaim the
--    storage and per-write maintenance cost.
DROP INDEX IF EXISTS idx_processed_companies_gin;
DROP INDEX IF EXISTS idx_processed_investors_gin;

-- 4. Drop the old feed ordering index replaced by idx_processed_feed_order.
DROP INDEX IF EXISTS idx_processed_fi_score;
