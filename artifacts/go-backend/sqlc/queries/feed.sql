-- name: GetFeedItems :many
SELECT id, title, summary, COALESCE(key_points,'[]') AS key_points,
       source_url, source, source_type,
       region, category, sentiment,
       sentiment_score, relevance_score, fi_score,
       companies, investors, amount, currency, round_type, valuation,
       COALESCE(coverage_count, 1) AS coverage_count,
       also_sources, ai_model_used,
       published_at, fetched_at
FROM processed_items
ORDER BY fi_score DESC NULLS LAST, published_at DESC NULLS LAST
LIMIT $1 OFFSET $2;

-- name: GetFeedItemByID :one
SELECT id, title, summary, COALESCE(key_points,'[]') AS key_points,
       source_url, source, source_type,
       region, category, sentiment,
       sentiment_score, relevance_score, fi_score,
       companies, investors, amount, currency, round_type, valuation,
       COALESCE(coverage_count, 1) AS coverage_count,
       also_sources, ai_model_used,
       published_at, fetched_at
FROM processed_items
WHERE id = $1;

-- name: GetFeedStats :one
SELECT
    COUNT(*)                                           AS total_articles,
    COUNT(*) FILTER (WHERE fetched_at >= $1)           AS today_articles,
    COUNT(*) FILTER (WHERE region = 'india')           AS india_articles,
    COUNT(*) FILTER (WHERE region = 'global')          AS global_articles
FROM processed_items;

-- name: GetLastFetchedAt :one
SELECT fetched_at FROM processed_items ORDER BY fetched_at DESC LIMIT 1;

-- name: GetCategoryCountsSince :many
SELECT category, COUNT(*) AS cnt
FROM processed_items
WHERE fetched_at >= $1 AND category IS NOT NULL
GROUP BY category;

-- name: GetAIModelBreakdownSince :many
SELECT ai_model_used, COUNT(*) AS cnt
FROM processed_items
WHERE fetched_at >= $1 AND ai_model_used IS NOT NULL
GROUP BY ai_model_used;

-- name: GetUnprocessedCount :one
SELECT COUNT(*) FROM raw_items WHERE processed = FALSE;

-- name: SearchFeedItems :many
SELECT id, title, summary, COALESCE(key_points,'[]') AS key_points,
       source_url, source, source_type,
       region, category, sentiment,
       sentiment_score, relevance_score, fi_score,
       companies, investors, amount, currency, round_type, valuation,
       COALESCE(coverage_count, 1) AS coverage_count,
       also_sources, ai_model_used,
       published_at, fetched_at
FROM processed_items
WHERE title    ILIKE $1
   OR summary  ILIKE $1
   OR companies::text ILIKE $1
   OR investors::text ILIKE $1
ORDER BY fi_score DESC NULLS LAST
LIMIT $2;

-- name: GetDigestArticles :many
SELECT id, title, summary, COALESCE(key_points,'[]') AS key_points,
       source_url, source, source_type,
       region, category, sentiment,
       sentiment_score, relevance_score, fi_score,
       companies, investors, amount, currency, round_type, valuation,
       COALESCE(coverage_count, 1) as coverage_count,
       also_sources, ai_model_used,
       published_at, fetched_at
FROM processed_items
WHERE fetched_at >= $1
ORDER BY fi_score DESC NULLS LAST
LIMIT 25;

-- name: GetTrendingEntities :many
SELECT companies, investors, category, fetched_at
FROM processed_items
WHERE fetched_at >= $1
LIMIT 800;
