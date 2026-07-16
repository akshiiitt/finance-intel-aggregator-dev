package handlers

import (
	"context"
	"errors"
	"math"
	"net/http"

	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// QuotaProvider is implemented by the AI worker to expose live API quota counts.
// Used by GET /api/feed/stats to include quota in the response — matches the
// Node.js response shape exactly.
type QuotaProvider interface {
	GetQuota() (gemini, groq8b, groq70b int64)
}

// FeedHandler handles all /api/feed/* routes.
type FeedHandler struct {
	pool  *pgxpool.Pool
	quota QuotaProvider // optional; nil when AI worker not yet available
}

// NewFeedHandler creates a feed handler.
func NewFeedHandler(pool *pgxpool.Pool) *FeedHandler {
	return &FeedHandler{pool: pool}
}

func (h *FeedHandler) getLiveUSDINRRate(ctx context.Context) float64 {
	var rate float64
	err := h.pool.QueryRow(ctx, `
		SELECT price::float8
		FROM market_snapshots
		WHERE symbol = 'USDINR=X'
		ORDER BY captured_at DESC
		LIMIT 1
	`).Scan(&rate)
	if err != nil || rate <= 0 || math.IsNaN(rate) {
		return 83.0 // fallback
	}
	return rate
}

// WithQuota attaches a live quota provider to the feed handler.
// Chain this after NewFeedHandler once the AI worker is available.
func (h *FeedHandler) WithQuota(q QuotaProvider) *FeedHandler {
	h.quota = q
	return h
}

// feedItem is the JSON shape returned by every feed endpoint.
// Matches the TypeScript mapItem() function exactly.
type feedItem struct {
	ID             int64    `json:"id"`
	Title          string   `json:"title"`
	Summary        *string  `json:"summary"`
	KeyPoints      string   `json:"keyPoints"`
	SourceURL      string   `json:"sourceUrl"`
	Source         string   `json:"source"`
	SourceType     *string  `json:"sourceType"`
	Region         *string  `json:"region"`
	Category       *string  `json:"category"`
	Sentiment      *string  `json:"sentiment"`
	SentimentScore *float64 `json:"sentimentScore"`
	RelevanceScore *float64 `json:"relevanceScore"`
	FiScore        *float64 `json:"fiScore"`
	Companies      *string  `json:"companies"`
	Investors      *string  `json:"investors"`
	Amount         *float64 `json:"amount"`
	Currency       *string  `json:"currency"`
	RoundType      *string  `json:"roundType"`
	Valuation      *float64 `json:"valuation"`
	CoverageCount  int      `json:"coverageCount"`
	AlsoSources    *string  `json:"alsoSources"`
	AIModelUsed    *string  `json:"aiModelUsed"`
	PublishedAt    *string  `json:"publishedAt"`
	FetchedAt      string   `json:"fetchedAt"`
}

const feedSelectCols = `
        SELECT id, title, summary, COALESCE(key_points,'[]') as key_points,
               source_url, source, source_type,
               region, category, sentiment, sentiment_score::float8, relevance_score::float8, fi_score::float8,
               jsonb_to_csv(companies) as companies, jsonb_to_csv(investors) as investors,
               amount::float8, currency, round_type, valuation::float8,
               COALESCE(coverage_count,1) as coverage_count, also_sources, ai_model_used,
               to_char(published_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"') as published_at,
               to_char(fetched_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"') as fetched_at
        FROM processed_items`

func scanFeedItem(rows interface{ Scan(...any) error }) (feedItem, error) {
	var it feedItem
	err := rows.Scan(
		&it.ID, &it.Title, &it.Summary, &it.KeyPoints,
		&it.SourceURL, &it.Source, &it.SourceType,
		&it.Region, &it.Category, &it.Sentiment, &it.SentimentScore, &it.RelevanceScore, &it.FiScore,
		&it.Companies, &it.Investors, &it.Amount, &it.Currency, &it.RoundType, &it.Valuation,
		&it.CoverageCount, &it.AlsoSources, &it.AIModelUsed,
		&it.PublishedAt, &it.FetchedAt,
	)
	return it, err
}

// GetFeed handles GET /api/feed
// Query params: limit, offset, region, category, sort, minAmountCr
func (h *FeedHandler) GetFeed(c *gin.Context) {
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	if limit > 100 {
		limit = 100
	}
	// Cap offset: without an upper bound, ?offset=50000000 forces Postgres
	// to walk and discard tens of millions of rows per request on the
	// scarce pooler connections. The feed only retains a 7-day window.
	if offset > 10000 {
		offset = 10000
	}

	region := c.Query("region")
	category := c.Query("category")
	sort := c.DefaultQuery("sort", "fiscore")
	minAmountCr := floatQuery(c, "minAmountCr", 0)

	rate := h.getLiveUSDINRRate(c.Request.Context())
	rateStr := strconv.FormatFloat(rate, 'f', 2, 64)

	args := []interface{}{}
	where := []string{"1=1"}
	argN := 1

	if region != "" && region != "both" {
		where = append(where, "region = $"+strconv.Itoa(argN))
		args = append(args, region)
		argN++
	}
	if category != "" && category != "all" {
		where = append(where, "category = $"+strconv.Itoa(argN))
		args = append(args, category)
		argN++
	}
	if minAmountCr > 0 {
		minAmountRaw := minAmountCr * 10_000_000 // crores to absolute INR
		where = append(where, "((currency = 'INR' AND amount >= $"+strconv.Itoa(argN)+") OR (currency != 'INR' AND amount * "+rateStr+" >= $"+strconv.Itoa(argN)+"))")
		args = append(args, minAmountRaw)
		argN++
	}

	whereClause := strings.Join(where, " AND ")

	var orderBy string
	switch sort {
	case "recency":
		orderBy = "published_at DESC NULLS LAST, fi_score DESC NULLS LAST"
	case "amount":
		orderBy = "CASE WHEN currency = 'INR' THEN amount ELSE amount * "+rateStr+" END DESC NULLS LAST, fi_score DESC NULLS LAST"
	default: // "fiscore"
		orderBy = "fi_score DESC NULLS LAST, published_at DESC NULLS LAST"
	}

	// Count total for pagination
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	var total int64
	if err := h.pool.QueryRow(c.Request.Context(),
		"SELECT COUNT(*) FROM processed_items WHERE "+whereClause, countArgs...).Scan(&total); err != nil {
		// Don't silently report total=0 next to a non-empty page — that
		// corrupts pagination. Log it; the items query below still runs.
		log.Warn().Err(err).Msg("feed: count query failed")
	}

	// Main query
	args = append(args, limit, offset)
	query := feedSelectCols + `
                WHERE ` + whereClause + `
                ORDER BY ` + orderBy + `
                LIMIT $` + strconv.Itoa(argN) + ` OFFSET $` + strconv.Itoa(argN+1)

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	items := []feedItem{}
	for rows.Next() {
		if it, err := scanFeedItem(rows); err == nil {
			items = append(items, it)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

// GetStats handles GET /api/feed/stats
// Returns aggregate stats matching the TypeScript /api/feed/stats response shape exactly.
func (h *FeedHandler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()


	var totalArticles, todayCount, indiaCount, globalCount, unprocessedCount int64

	// One pass over processed_items with conditional aggregates instead of
	// four separate full-table COUNT round-trips.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := h.pool.QueryRow(gctx, `
                SELECT total_articles, today_count, india_count, global_count, unprocessed_count
                FROM feed_stats_mv`).Scan(&totalArticles, &todayCount, &indiaCount, &globalCount, &unprocessedCount); err != nil {
			log.Error().Err(err).Msg("stats: failed to query feed_stats_mv")
			return err
		}
		return nil
	})
	_ = g.Wait()

	// Last fetch time
	var lastFetch *string
	row := h.pool.QueryRow(ctx, `
                SELECT to_char(last_fetch,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
                FROM feed_stats_mv`)
	if err := row.Scan(&lastFetch); err != nil && err != pgx.ErrNoRows {
		log.Error().Err(err).Msg("stats: failed to scan last fetch time from mv")
	}

	// Category counts (last 24h)
	catRows, err := h.pool.Query(ctx, `SELECT category, count FROM feed_category_stats_mv`)
	if err != nil {
		log.Error().Err(err).Msg("stats: failed to query category counts")
	}
	categoryCounts := map[string]int64{}
	if catRows != nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat string
			var cnt int64
			if err := catRows.Scan(&cat, &cnt); err == nil {
				categoryCounts[cat] = cnt
			}
		}
	}

	// AI model breakdown (today)
	aiRows, err := h.pool.Query(ctx, `SELECT ai_model_used, count FROM feed_ai_stats_mv`)
	if err != nil {
		log.Error().Err(err).Msg("stats: failed to query ai model breakdown")
	}
	aiBreakdown := map[string]int64{}
	if aiRows != nil {
		defer aiRows.Close()
		for aiRows.Next() {
			var model string
			var cnt int64
			if err := aiRows.Scan(&model, &cnt); err == nil {
				aiBreakdown[model] = cnt
			}
		}
	}

	resp := gin.H{
		"totalArticles":    totalArticles,
		"todayArticles":    todayCount,
		"indiaArticles":    indiaCount,
		"globalArticles":   globalCount,
		"lastFetch":        lastFetch,
		"categoryCounts":   categoryCounts,
		"unprocessedQueue": unprocessedCount,
		"aiBreakdown":      aiBreakdown,
	}

	// Include live quota — matches Node.js response shape exactly:
	// { quota: { geminiUsed, geminiLimit, groq8bUsed, groq8bLimit, groq70bUsed, groq70bLimit } }
	if h.quota != nil {
		g, g8, g70 := h.quota.GetQuota()
		resp["quota"] = gin.H{
			"geminiUsed":   g,
			"geminiLimit":  1500,
			"groq8bUsed":   g8,
			"groq8bLimit":  14400,
			"groq70bUsed":  g70,
			"groq70bLimit": 1000,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetTrending handles GET /api/feed/trending
// Returns hot entities (companies/investors) ranked by frequency + recency.
func (h *FeedHandler) GetTrending(c *gin.Context) {
	ctx := c.Request.Context()

	rows, err := h.pool.Query(ctx, `SELECT term, category, count, is_hot FROM feed_trending_mv ORDER BY score DESC LIMIT 20`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	type topicItem struct {
		Term     string `json:"term"`
		Count    int    `json:"count"`
		IsHot    bool   `json:"isHot"`
		Category string `json:"category"`
	}
	topics := []topicItem{}

	for rows.Next() {
		var t topicItem
		if err := rows.Scan(&t.Term, &t.Category, &t.Count, &t.IsHot); err == nil {
			topics = append(topics, t)
		}
	}

	c.JSON(http.StatusOK, gin.H{"topics": topics})
}

// splitEntities splits comma-separated companies and investors strings into a flat slice.
func splitEntities(companies, investors *string) []string {
	var out []string
	if companies != nil && *companies != "" {
		for _, s := range strings.Split(*companies, ",") {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
	}
	if investors != nil && *investors != "" {
		for _, s := range strings.Split(*investors, ",") {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// GetDigest handles GET /api/feed/digest
// Returns a digest + top articles for today. If the digest worker
// (internal/worker/digest) has already generated today's real AI briefing,
// its content replaces the templated markdown below — same response shape
// either way, so the frontend contract never changes.
func (h *FeedHandler) GetDigest(c *gin.Context) {
	ctx := c.Request.Context()
	today := time.Now().Truncate(24 * time.Hour)

	var generatedContent string
	var topStoryIDs []int64
	hasGenerated := h.pool.QueryRow(ctx, `
                SELECT content, top_story_ids FROM daily_digests WHERE digest_date = $1
        `, time.Now().UTC().Format("2006-01-02")).Scan(&generatedContent, &topStoryIDs) == nil

	rows, err := h.pool.Query(ctx, feedSelectCols+`
                WHERE fetched_at >= $1
                ORDER BY fi_score DESC NULLS LAST
                LIMIT 25`, today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var topArticles []feedItem
	for rows.Next() {
		if it, err := scanFeedItem(rows); err == nil {
			topArticles = append(topArticles, it)
		}
	}

	date := time.Now().Format("Monday, 02 January 2006")

	// Group by category
	byCategory := map[string][]feedItem{}
	for _, a := range topArticles {
		cat := ""
		if a.Category != nil {
			cat = *a.Category
		}
		byCategory[cat] = append(byCategory[cat], a)
	}

	var sections []string

	appendSection := func(header string, items []feedItem, max int) {
		if len(items) == 0 {
			return
		}
		sections = append(sections, header)
		if len(items) > max {
			items = items[:max]
		}
		for _, a := range items {
			amt := ""
			if a.Amount != nil && a.Currency != nil {
				if *a.Currency == "INR" {
					amt = " — ₹" + strconv.FormatInt(int64(*a.Amount/10_000_000), 10) + " Cr"
				} else {
					amt = " — $" + strconv.FormatInt(int64(*a.Amount/1_000_000), 10) + "M"
				}
			}
			sections = append(sections, "• **"+a.Title+"**"+amt)
			if a.Summary != nil && *a.Summary != "" {
				sections = append(sections, "  "+*a.Summary)
			}
		}
	}

	appendSection("## 💰 FUNDING RADAR", byCategory["funding"], 5)
	appendSection("\n## 📈 MARKETS", byCategory["markets"], 3)
	appendSection("\n## 🏛️ POLICY & REGULATORY", byCategory["policy"], 3)
	appendSection("\n## 🔔 IPO WATCH", byCategory["ipo"], 3)
	appendSection("\n## 🤝 M&A", byCategory["mergers"], 2)
	appendSection("\n## 📊 EARNINGS", byCategory["earnings"], 2)

	// Global articles
	var globalItems []feedItem
	for _, a := range topArticles {
		if a.Region != nil && *a.Region == "global" {
			globalItems = append(globalItems, a)
		}
	}
	appendSection("\n## 🌐 GLOBAL SIGNALS", globalItems, 3)

	content := strings.Join(sections, "\n")
	if content == "" {
		content = "No articles processed yet for today. Check back after the first feed fetch completes."
	}

	// If the digest worker has already written today's real AI briefing,
	// it replaces the templated markdown above — same "content" key, same
	// "topArticles" shape, so nothing downstream needs to change.
	generated := hasGenerated
	if generated {
		content = generatedContent
	}

	c.JSON(http.StatusOK, gin.H{
		"date":        date,
		"content":     content,
		"topArticles": topArticles,
		"generated":   generated,
	})
}

// GetSearch handles GET /api/feed/search?q=...
// Full-text search across title, summary, companies, investors.
func (h *FeedHandler) GetSearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" || len(q) < 3 {
		c.JSON(http.StatusOK, gin.H{"items": []feedItem{}, "total": 0})
		return
	}

	limit := intQuery(c, "limit", 30)
	if limit > 50 {
		limit = 50
	}

	pattern := "%" + q + "%"
	rows, err := h.pool.Query(c.Request.Context(), feedSelectCols+`
                WHERE title ILIKE $1 OR summary ILIKE $1 OR companies::text ILIKE $1 OR investors::text ILIKE $1
                ORDER BY fi_score DESC NULLS LAST
                LIMIT $2`, pattern, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	items := []feedItem{}
	for rows.Next() {
		if it, err := scanFeedItem(rows); err == nil {
			items = append(items, it)
		}
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// GetFeedItem handles GET /api/feed/:id
func (h *FeedHandler) GetFeedItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	row := h.pool.QueryRow(c.Request.Context(), feedSelectCols+` WHERE id = $1`, id)
	it, err := scanFeedItem(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// A real DB/connection error must not masquerade as a 404.
		log.Error().Err(err).Int64("id", id).Msg("feed: get item failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	c.JSON(http.StatusOK, it)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func intQuery(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	// Every int query param here (limit/offset/hours/days) is non-negative;
	// a negative value would flow into LIMIT/OFFSET and make Postgres 500.
	if n < 0 {
		return def
	}
	return n
}

func floatQuery(c *gin.Context, key string, def float64) float64 {
	v := c.Query(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
