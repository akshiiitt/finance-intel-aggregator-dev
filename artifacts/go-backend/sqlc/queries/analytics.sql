-- name: GetAnalyticsOverview :one
SELECT
    COUNT(*)                                                              AS total_articles,
    COUNT(*) FILTER (WHERE fetched_at >= $1)                             AS today_count,
    COUNT(*) FILTER (WHERE fetched_at >= $2 AND fetched_at < $1)         AS yesterday_count,
    COUNT(*) FILTER (WHERE fetched_at >= $3)                             AS week_total,
    COALESCE(AVG(fi_score::float8) FILTER (WHERE fetched_at >= $1), 0)  AS avg_fi_score,
    COALESCE(MAX(fi_score::float8) FILTER (WHERE fetched_at >= $1), 0)  AS top_fi_score
FROM processed_items;

-- name: GetSentimentTrend :many
SELECT
    date_trunc('hour', fetched_at) AS hour,
    sentiment,
    COUNT(*) AS cnt
FROM processed_items
WHERE fetched_at >= NOW() - INTERVAL '24 hours'
  AND sentiment IS NOT NULL
GROUP BY hour, sentiment
ORDER BY hour ASC;

-- name: GetFundingLeaders :many
SELECT id, title, companies, investors,
       amount, currency, round_type, fi_score,
       published_at, source_url
FROM processed_items
WHERE category IN ('funding', 'mergers')
  AND amount IS NOT NULL
  AND fetched_at >= NOW() - INTERVAL '30 days'
ORDER BY amount DESC NULLS LAST
LIMIT 20;

-- name: GetTimeline :many
SELECT
    date_trunc('hour', fetched_at)                                AS hour,
    COUNT(*)                                                      AS total,
    COUNT(*) FILTER (WHERE region = 'india')                      AS india,
    COUNT(*) FILTER (WHERE region = 'global')                     AS global,
    COALESCE(AVG(fi_score::float8), 0)                           AS avg_fi
FROM processed_items
WHERE fetched_at >= NOW() - INTERVAL '7 days'
GROUP BY hour
ORDER BY hour ASC;
