CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 1. Trigram indexes (already created in 000001 and 000005)

-- 2. Article Entities Junction Table for Fast O(1) Lookups
CREATE TABLE IF NOT EXISTS article_entities (
    article_id BIGINT NOT NULL REFERENCES processed_items(id) ON DELETE CASCADE,
    entity_name TEXT NOT NULL,
    entity_type TEXT NOT NULL, -- 'company' or 'investor'
    PRIMARY KEY (article_id, entity_name, entity_type)
);

CREATE INDEX IF NOT EXISTS idx_article_entities_name ON article_entities (entity_name);

-- 3. Materialized Views for Stats & Trending
CREATE MATERIALIZED VIEW IF NOT EXISTS feed_stats_mv AS
SELECT
    (SELECT COUNT(*) FROM processed_items) as total_articles,
    (SELECT COUNT(*) FROM processed_items WHERE fetched_at >= date_trunc('day', now() AT TIME ZONE 'UTC')) as today_count,
    (SELECT COUNT(*) FROM processed_items WHERE fetched_at >= date_trunc('day', now() AT TIME ZONE 'UTC') AND region = 'india') as india_count,
    (SELECT COUNT(*) FROM processed_items WHERE fetched_at >= date_trunc('day', now() AT TIME ZONE 'UTC') AND region = 'global') as global_count,
    (SELECT COUNT(*) FROM raw_items WHERE processed = FALSE) as unprocessed_count,
    (SELECT MAX(fetched_at) FROM processed_items) as last_fetch;

CREATE UNIQUE INDEX IF NOT EXISTS idx_feed_stats_mv_uniq ON feed_stats_mv (total_articles);

CREATE MATERIALIZED VIEW IF NOT EXISTS feed_category_stats_mv AS
SELECT category, COUNT(*) as count
FROM processed_items
WHERE fetched_at >= now() - interval '24 hours' AND category IS NOT NULL
GROUP BY category;
CREATE UNIQUE INDEX IF NOT EXISTS idx_feed_category_stats_mv_cat ON feed_category_stats_mv (category);

CREATE MATERIALIZED VIEW IF NOT EXISTS feed_ai_stats_mv AS
SELECT ai_model_used, COUNT(*) as count
FROM processed_items
WHERE fetched_at >= date_trunc('day', now() AT TIME ZONE 'UTC') AND ai_model_used IS NOT NULL
GROUP BY ai_model_used;
CREATE UNIQUE INDEX IF NOT EXISTS idx_feed_ai_stats_mv_model ON feed_ai_stats_mv (ai_model_used);

CREATE MATERIALIZED VIEW IF NOT EXISTS feed_trending_mv AS
WITH recent AS (
    SELECT entity_name, e.article_id, p.fetched_at, p.category
    FROM article_entities e
    JOIN processed_items p ON e.article_id = p.id
    WHERE p.fetched_at >= now() - interval '48 hours'
),
hot_set AS (
    SELECT entity_name
    FROM recent
    WHERE fetched_at >= now() - interval '1 hour'
    GROUP BY entity_name
)
SELECT 
    r.entity_name as term,
    MAX(r.category) as category,
    COUNT(*) as count,
    CASE WHEN h.entity_name IS NOT NULL THEN true ELSE false END as is_hot,
    (COUNT(*) + CASE WHEN h.entity_name IS NOT NULL THEN 3 ELSE 0 END) as score
FROM recent r
LEFT JOIN hot_set h ON r.entity_name = h.entity_name
WHERE length(r.entity_name) > 2
GROUP BY r.entity_name, h.entity_name
ORDER BY score DESC
LIMIT 20;

CREATE UNIQUE INDEX IF NOT EXISTS idx_feed_trending_mv_term ON feed_trending_mv (term);
