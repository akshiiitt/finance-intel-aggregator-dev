DROP MATERIALIZED VIEW IF EXISTS feed_trending_mv;
DROP MATERIALIZED VIEW IF EXISTS feed_ai_stats_mv;
DROP MATERIALIZED VIEW IF EXISTS feed_category_stats_mv;
DROP MATERIALIZED VIEW IF EXISTS feed_stats_mv;

DROP TABLE IF EXISTS article_entities;

-- Not dropping extension pg_trgm as it might be used elsewhere
