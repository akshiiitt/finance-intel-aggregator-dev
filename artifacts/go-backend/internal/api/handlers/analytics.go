package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// AnalyticsHandler handles all /api/analytics/* routes.
type AnalyticsHandler struct {
	pool *pgxpool.Pool
}

// NewAnalyticsHandler creates an analytics handler.
func NewAnalyticsHandler(pool *pgxpool.Pool) *AnalyticsHandler {
	return &AnalyticsHandler{pool: pool}
}

// getLiveUSDCroreFactor fetches the latest USDINR=X price from market_snapshots
// and returns the crore conversion factor:  (usdINR / 10).
//
//	$1 M  ×  usdINR  INR/$  =  usdINR × 10⁶ INR  =  (usdINR/10) crore
//
// Falls back to 8.4 (84 INR/USD) if the market worker has not run yet.
func (h *AnalyticsHandler) getLiveUSDCroreFactor(ctx context.Context) float64 {
	var rate float64
	err := h.pool.QueryRow(ctx, `
		SELECT price::float8
		FROM market_snapshots
		WHERE symbol = 'USDINR=X'
		ORDER BY captured_at DESC
		LIMIT 1
	`).Scan(&rate)
	if err != nil || rate <= 0 || math.IsNaN(rate) {
		return 8.4 // 84 INR/USD ÷ 10 = 8.4 crore per $1M
	}
	return rate / 10.0
}

// usdConvSQL returns a SQL CASE expression that converts an `amount` column to
// INR crore using the live rate.  factor is injected as a literal float — safe
// because it comes from a trusted DB query, not user input.
func usdConvSQL(factor float64) string {
	return fmt.Sprintf(
		"CASE WHEN currency = 'INR' THEN amount::numeric / 10000000 "+
			"WHEN currency = 'USD' THEN amount::numeric / 1000000 * %.6f "+
			"ELSE 0 END",
		factor,
	)
}

// ── GET /api/analytics/overview ───────────────────────────────────────────────
// Returns dashboard summary cards — exactly mirrors the Node.js shape.
func (h *AnalyticsHandler) GetOverview(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now()
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since24h := now.Add(-24 * time.Hour)
	since90d := now.Add(-90 * 24 * time.Hour)
	since30d := now.Add(-30 * 24 * time.Hour)

	// Fetch live FX rate first (fast — single-row indexed lookup)
	factor := h.getLiveUSDCroreFactor(ctx)

	type result struct {
		totalArticles    int64
		todayArticles    int64
		indiaArticles    int64
		globalArticles   int64
		dealsFound       int64
		totalFundingCr   float64
		avgFiScore       float64
		sourcesActive24h int64
		unprocessedQueue int64
		avgCoverageCount float64
	}
	var r result

	// All processed_items aggregates in ONE pass with conditional aggregates.
	// Previously this fanned out 10 goroutines each acquiring its own pooler
	// connection — a single overview request grabbed most of the free-tier
	// connection budget and starved every other endpoint under light load.
	overviewQ := fmt.Sprintf(`
		SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE fetched_at >= $1),
		    COUNT(*) FILTER (WHERE region = 'india'),
		    COUNT(*) FILTER (WHERE region = 'global'),
		    COUNT(*) FILTER (WHERE amount IS NOT NULL AND fetched_at > $3),
		    COALESCE(SUM(%s) FILTER (WHERE amount IS NOT NULL AND fetched_at > $4), 0),
		    COALESCE(AVG(fi_score::numeric) FILTER (WHERE fetched_at > $1), 0),
		    COUNT(DISTINCT source) FILTER (WHERE fetched_at > $2),
		    COALESCE(AVG(coverage_count) FILTER (WHERE fetched_at > $1), 1)
		FROM processed_items`, usdConvSQL(factor))

	if err := h.pool.QueryRow(ctx, overviewQ, todayMidnight, since24h, since30d, since90d).Scan(
		&r.totalArticles, &r.todayArticles, &r.indiaArticles, &r.globalArticles,
		&r.dealsFound, &r.totalFundingCr, &r.avgFiScore, &r.sourcesActive24h,
		&r.avgCoverageCount,
	); err != nil {
		log.Error().Err(err).Msg("analytics: failed to load overview stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load overview statistics"})
		return
	}

	// Unprocessed queue lives in raw_items — one more small round-trip.
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM raw_items WHERE processed = FALSE`).Scan(&r.unprocessedQueue); err != nil {
		log.Error().Err(err).Msg("analytics: failed to load unprocessed queue count")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load overview statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"totalArticles":            r.totalArticles,
		"todayArticles":            r.todayArticles,
		"indiaArticles":            r.indiaArticles,
		"globalArticles":           r.globalArticles,
		"dealsFound":               r.dealsFound,
		"totalFundingDiscoveredCr": math.Round(r.totalFundingCr),
		"avgFiScore":               math.Round(r.avgFiScore*10) / 10,
		"sourcesActive24h":         r.sourcesActive24h,
		"unprocessedQueue":         r.unprocessedQueue,
		"avgCoverageCount":         math.Round(r.avgCoverageCount*10) / 10,
	})
}

// ── GET /api/analytics/sentiment ──────────────────────────────────────────────
// Returns 7-day sentiment breakdown grouped by category + overall aggregate.
// Mirrors Node.js shape: {byCategory: [...], overall: {...}}
func (h *AnalyticsHandler) GetSentimentTrend(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT
		    COALESCE(category, 'general') as category,
		    COUNT(CASE WHEN sentiment = 'bullish' THEN 1 END)::bigint  as bullish,
		    COUNT(CASE WHEN sentiment = 'bearish' THEN 1 END)::bigint  as bearish,
		    COUNT(CASE WHEN sentiment = 'neutral' THEN 1 END)::bigint  as neutral
		FROM processed_items
		WHERE fetched_at > NOW() - INTERVAL '7 days'
		  AND category IS NOT NULL
		GROUP BY category
		ORDER BY (
		    COUNT(CASE WHEN sentiment = 'bullish' THEN 1 END) +
		    COUNT(CASE WHEN sentiment = 'bearish' THEN 1 END) +
		    COUNT(CASE WHEN sentiment = 'neutral' THEN 1 END)
		) DESC
		LIMIT 12
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	type catRow struct {
		Category string `json:"category"`
		Bullish  int64  `json:"bullish"`
		Bearish  int64  `json:"bearish"`
		Neutral  int64  `json:"neutral"`
	}

	byCategory := []catRow{}
	overall := catRow{Category: "all"}

	for rows.Next() {
		var r catRow
		if err := rows.Scan(&r.Category, &r.Bullish, &r.Bearish, &r.Neutral); err != nil {
			continue
		}
		byCategory = append(byCategory, r)
		overall.Bullish += r.Bullish
		overall.Bearish += r.Bearish
		overall.Neutral += r.Neutral
	}

	c.JSON(http.StatusOK, gin.H{
		"byCategory": byCategory,
		"overall":    overall,
	})
}

// ── GET /api/analytics/funding ─────────────────────────────────────────────────
// Returns rich funding analytics — mirrors Node.js shape:
// {bySector, byRound, monthly, topDeals, sourceActivity}
// All INR crore conversions use the live USD/INR rate from market_snapshots.
func (h *AnalyticsHandler) GetFundingLeaders(c *gin.Context) {
	ctx := c.Request.Context()
	factor := h.getLiveUSDCroreFactor(ctx)
	conv := usdConvSQL(factor)

	// ── bySector ─────────────────────────────────────────────────────────────
	type sectorRow struct {
		Category   string  `json:"category"`
		DealCount  int     `json:"dealCount"`
		TotalAmtCr float64 `json:"totalAmountCr"`
	}
	bySector := []sectorRow{}

	sectorRows, err := h.pool.Query(ctx, fmt.Sprintf(`
		SELECT
		    COALESCE(category, 'general') as category,
		    COUNT(*)::integer             as deal_count,
		    COALESCE(SUM(%s), 0)         as total_amount_cr
		FROM processed_items
		WHERE amount IS NOT NULL
		  AND fetched_at > NOW() - INTERVAL '30 days'
		GROUP BY category
		ORDER BY total_amount_cr DESC
	`, conv))
	if err == nil {
		defer sectorRows.Close()
		for sectorRows.Next() {
			var r sectorRow
			var amt float64
			if scanErr := sectorRows.Scan(&r.Category, &r.DealCount, &amt); scanErr == nil {
				r.TotalAmtCr = math.Round(amt)
				bySector = append(bySector, r)
			}
		}
	}

	// ── byRound ──────────────────────────────────────────────────────────────
	type roundRow struct {
		RoundType  string  `json:"roundType"`
		Count      int     `json:"count"`
		TotalAmtCr float64 `json:"totalAmountCr"`
	}
	byRound := []roundRow{}

	roundRows, err := h.pool.Query(ctx, fmt.Sprintf(`
		SELECT
		    COALESCE(round_type, 'Other') as round_type,
		    COUNT(*)::integer             as cnt,
		    COALESCE(SUM(%s), 0)         as total_amount_cr
		FROM processed_items
		WHERE amount IS NOT NULL AND round_type IS NOT NULL
		  AND fetched_at > NOW() - INTERVAL '90 days'
		GROUP BY round_type
		ORDER BY cnt DESC
		LIMIT 10
	`, conv))
	if err == nil {
		defer roundRows.Close()
		for roundRows.Next() {
			var r roundRow
			var amt float64
			if scanErr := roundRows.Scan(&r.RoundType, &r.Count, &amt); scanErr == nil {
				r.TotalAmtCr = math.Round(amt)
				byRound = append(byRound, r)
			}
		}
	}

	// ── monthly ───────────────────────────────────────────────────────────────
	type monthRow struct {
		Month        string  `json:"month"`
		ArticleCount int     `json:"articleCount"`
		DealCount    int     `json:"dealCount"`
		FundingCr    float64 `json:"fundingCr"`
	}
	monthly := []monthRow{}

	monthRows, err := h.pool.Query(ctx, fmt.Sprintf(`
		SELECT
		    TO_CHAR(DATE_TRUNC('month', COALESCE(published_at, fetched_at)), 'Mon YYYY') as month,
		    COUNT(*)::integer                                                              as article_count,
		    COUNT(CASE WHEN amount IS NOT NULL THEN 1 END)::integer                      as deal_count,
		    COALESCE(SUM(
		        CASE WHEN amount IS NOT NULL THEN (%s) ELSE 0 END
		    ), 0)                                                                         as funding_cr
		FROM processed_items
		WHERE COALESCE(published_at, fetched_at) > NOW() - INTERVAL '6 months'
		GROUP BY DATE_TRUNC('month', COALESCE(published_at, fetched_at))
		ORDER BY DATE_TRUNC('month', COALESCE(published_at, fetched_at)) ASC
	`, conv))
	if err == nil {
		defer monthRows.Close()
		for monthRows.Next() {
			var r monthRow
			var fundingCr float64
			if scanErr := monthRows.Scan(&r.Month, &r.ArticleCount, &r.DealCount, &fundingCr); scanErr == nil {
				r.FundingCr = math.Round(fundingCr)
				monthly = append(monthly, r)
			}
		}
	}

	// ── topDeals ──────────────────────────────────────────────────────────────
	type dealRow struct {
		ID          int64    `json:"id"`
		Title       string   `json:"title"`
		Companies   *string  `json:"companies"`
		Investors   *string  `json:"investors"`
		Amount      *float64 `json:"amount"`
		Currency    *string  `json:"currency"`
		RoundType   *string  `json:"roundType"`
		Valuation   *float64 `json:"valuation"`
		Source      *string  `json:"source"`
		SourceURL   *string  `json:"sourceUrl"`
		FiScore     *float64 `json:"fiScore"`
		Category    *string  `json:"category"`
		PublishedAt *string  `json:"publishedAt"`
		FetchedAt   string   `json:"fetchedAt"`
	}
	topDeals := []dealRow{}

	dealRows, err := h.pool.Query(ctx, `
		SELECT
		    id, title, jsonb_to_csv(companies), jsonb_to_csv(investors),
		    amount::float8, currency, round_type, valuation::float8,
		    source, source_url, fi_score::float8, category,
		    to_char(published_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		    to_char(fetched_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM processed_items
		WHERE amount IS NOT NULL
		ORDER BY amount::numeric DESC, fi_score::numeric DESC NULLS LAST
		LIMIT 25
	`)
	if err == nil {
		defer dealRows.Close()
		for dealRows.Next() {
			var d dealRow
			if scanErr := dealRows.Scan(
				&d.ID, &d.Title, &d.Companies, &d.Investors,
				&d.Amount, &d.Currency, &d.RoundType, &d.Valuation,
				&d.Source, &d.SourceURL, &d.FiScore, &d.Category,
				&d.PublishedAt, &d.FetchedAt,
			); scanErr == nil {
				topDeals = append(topDeals, d)
			}
		}
	}

	// ── sourceActivity ────────────────────────────────────────────────────────
	type sourceRow struct {
		Source     string  `json:"source"`
		Count      int     `json:"count"`
		AvgFiScore float64 `json:"avgFiScore"`
	}
	sourceActivity := []sourceRow{}

	sourceRows, err := h.pool.Query(ctx, `
		SELECT source, COUNT(*)::integer as cnt,
		       COALESCE(AVG(fi_score::numeric), 0) as avg_fi
		FROM processed_items
		WHERE fetched_at > NOW() - INTERVAL '24 hours'
		GROUP BY source
		ORDER BY cnt DESC
		LIMIT 20
	`)
	if err == nil {
		defer sourceRows.Close()
		for sourceRows.Next() {
			var r sourceRow
			var avgFi float64
			if scanErr := sourceRows.Scan(&r.Source, &r.Count, &avgFi); scanErr == nil {
				r.AvgFiScore = math.Round(avgFi*10) / 10
				sourceActivity = append(sourceActivity, r)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"bySector":       bySector,
		"byRound":        byRound,
		"monthly":        monthly,
		"topDeals":       topDeals,
		"sourceActivity": sourceActivity,
	})
}

// ── GET /api/analytics/timeline ───────────────────────────────────────────────
// Returns daily article + funding totals for the last 30 days.
// Mirrors Node.js shape: {daily: [{day, count, indiaCount, globalCount, fundingCr}]}
// fundingCr is in INR crore converted at the live USD/INR rate.
func (h *AnalyticsHandler) GetTimeline(c *gin.Context) {
	ctx := c.Request.Context()
	factor := h.getLiveUSDCroreFactor(ctx)

	rows, err := h.pool.Query(ctx, fmt.Sprintf(`
		SELECT
		    TO_CHAR(DATE_TRUNC('day', COALESCE(published_at, fetched_at)), 'Mon DD') as day,
		    COUNT(*)::bigint                                                           as count,
		    COUNT(CASE WHEN region = 'india'  THEN 1 END)::bigint                    as india_count,
		    COUNT(CASE WHEN region = 'global' THEN 1 END)::bigint                    as global_count,
		    COALESCE(SUM(
		        CASE WHEN amount IS NOT NULL THEN (%s) ELSE 0 END
		    ), 0)                                                                     as funding_cr
		FROM processed_items
		WHERE COALESCE(published_at, fetched_at) > NOW() - INTERVAL '30 days'
		GROUP BY DATE_TRUNC('day', COALESCE(published_at, fetched_at))
		ORDER BY DATE_TRUNC('day', COALESCE(published_at, fetched_at)) ASC
	`, usdConvSQL(factor)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	type dayRow struct {
		Day         string  `json:"day"`
		Count       int64   `json:"count"`
		IndiaCount  int64   `json:"indiaCount"`
		GlobalCount int64   `json:"globalCount"`
		FundingCr   float64 `json:"fundingCr"`
	}

	daily := []dayRow{}
	for rows.Next() {
		var r dayRow
		var fundingCr float64
		if err := rows.Scan(&r.Day, &r.Count, &r.IndiaCount, &r.GlobalCount, &fundingCr); err != nil {
			continue
		}
		r.FundingCr = math.Round(fundingCr)
		daily = append(daily, r)
	}

	c.JSON(http.StatusOK, gin.H{"daily": daily})
}

// round2 rounds a float to 2 decimal places.
func round2(f float64) float64 {
	if f == 0 {
		return 0
	}
	return math.Round(f*100) / 100
}
