package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/financeintel/backend/internal/fxrates"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// DealsHandler handles /api/deals and /api/entity routes.
type DealsHandler struct {
	pool *pgxpool.Pool
}

// NewDealsHandler creates a deals handler.
func NewDealsHandler(pool *pgxpool.Pool) *DealsHandler {
	return &DealsHandler{pool: pool}
}

// processedItemOut is the full processed-item shape returned by /deals and /entity.
// Mirrors the TypeScript mapItem() function exactly.
type processedItemOut struct {
	ID             int64    `json:"id"`
	Title          string   `json:"title"`
	Summary        *string  `json:"summary"`
	KeyPoints      *string  `json:"keyPoints"`
	SourceURL      string   `json:"sourceUrl"`
	Source         *string  `json:"source"`
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
	CoverageCount  *int     `json:"coverageCount"`
	AlsoSources    *string  `json:"alsoSources"`
	AIModelUsed    *string  `json:"aiModelUsed"`
	PublishedAt    *string  `json:"publishedAt"`
	FetchedAt      string   `json:"fetchedAt"`
}

// scanProcessedItem scans a full processed_item row into processedItemOut.
// The SELECT must return columns in the exact order declared below.
func scanProcessedItem(rows interface {
	Scan(dest ...any) error
}) (processedItemOut, error) {
	var item processedItemOut
	err := rows.Scan(
		&item.ID, &item.Title, &item.Summary, &item.KeyPoints,
		&item.SourceURL, &item.Source, &item.SourceType, &item.Region,
		&item.Category, &item.Sentiment, &item.SentimentScore, &item.RelevanceScore,
		&item.FiScore, &item.Companies, &item.Investors,
		&item.Amount, &item.Currency, &item.RoundType, &item.Valuation,
		&item.CoverageCount, &item.AlsoSources, &item.AIModelUsed,
		&item.PublishedAt, &item.FetchedAt,
	)
	return item, err
}

const processedItemSelectCols = `
	id, title, summary, key_points,
	source_url, source, source_type, region,
	category, sentiment, sentiment_score::float8, relevance_score::float8,
	fi_score::float8, jsonb_to_csv(companies), jsonb_to_csv(investors),
	amount::float8, currency, round_type, valuation::float8,
	coverage_count, also_sources, ai_model_used,
	to_char(published_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
	to_char(fetched_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
`

// ── GET /api/deals ────────────────────────────────────────────────────────────
// Query params: roundType, minAmountCr, sort (amount|recency|fiscore), limit, offset, region, category
// Returns: {items: [...], total: N}
func (h *DealsHandler) GetDeals(c *gin.Context) {
	ctx := c.Request.Context()

	roundType := strings.TrimSpace(c.Query("roundType"))
	region := strings.TrimSpace(c.Query("region"))
	category := strings.TrimSpace(c.Query("category"))
	sort := strings.TrimSpace(c.Query("sort"))
	if sort == "" {
		sort = "amount"
	}
	limit := intQuery(c, "limit", 50)
	if limit > 100 {
		limit = 100
	}
	offset := intQuery(c, "offset", 0)
	// Cap offset to bound deep-pagination cost on the shared pooler.
	if offset > 10000 {
		offset = 10000
	}

	var minAmountRaw *float64
	if raw := strings.TrimSpace(c.Query("minAmountCr")); raw != "" {
		var f float64
		if _, err := fmt.Sscanf(raw, "%f", &f); err == nil && f > 0 {
			v := f * 10_000_000 // crore → absolute INR
			minAmountRaw = &v
		}
	}

	// Build WHERE clauses
	conditions := []string{"amount IS NOT NULL"}
	args := []interface{}{}
	argN := 1

	if roundType != "" && roundType != "all" {
		conditions = append(conditions, fmt.Sprintf("round_type = $%d", argN))
		args = append(args, roundType)
		argN++
	}
	if region != "" && region != "both" {
		conditions = append(conditions, fmt.Sprintf("region = $%d", argN))
		args = append(args, region)
		argN++
	}
	if category != "" && category != "all" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argN))
		args = append(args, category)
		argN++
	}
	if minAmountRaw != nil {
		conditions = append(conditions, fmt.Sprintf("amount::numeric >= $%d", argN))
		args = append(args, *minAmountRaw)
		argN++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// ORDER BY
	var orderBy string
	switch sort {
	case "recency":
		orderBy = "published_at DESC NULLS LAST, fi_score::numeric DESC NULLS LAST"
	case "fiscore":
		orderBy = "fi_score::numeric DESC NULLS LAST"
	default:
		orderBy = "amount::numeric DESC NULLS LAST, fi_score::numeric DESC NULLS LAST"
	}

	// Count query
	var total int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := h.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM processed_items "+where,
		countArgs...,
	).Scan(&total); err != nil {
		log.Warn().Err(err).Msg("deals: count query failed")
	}

	// Data query
	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, limit, offset)
	limitN := argN
	offsetN := argN + 1

	query := fmt.Sprintf(`
		SELECT %s FROM processed_items
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, processedItemSelectCols, where, orderBy, limitN, offsetN)

	rows, err := h.pool.Query(ctx, query, dataArgs...)
	if err != nil {
		log.Error().Err(err).Msg("deals: query failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	items := []processedItemOut{}
	for rows.Next() {
		item, err := scanProcessedItem(rows)
		if err == nil {
			items = append(items, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// ── GET /api/entity?name=Zepto ─────────────────────────────────────────────
// Returns all articles mentioning the entity + sentiment stats + co-mentioned companies.
func (h *DealsHandler) GetEntity(c *gin.Context) {
	ctx := c.Request.Context()
	name := strings.TrimSpace(c.Query("name"))
	if name == "" || len(name) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name query param required (min 2 chars)"})
		return
	}
	limit := intQuery(c, "limit", 40)
	if limit > 80 {
		limit = 80
	}

	pattern := "%" + name + "%"

	// Articles mentioning the entity
	articleRows, err := h.pool.Query(ctx, `
		SELECT p.id, p.title, p.summary, p.key_points,
		       p.source_url, p.source, p.source_type, p.region,
		       p.category, p.sentiment, p.sentiment_score::float8, p.relevance_score::float8,
		       p.fi_score::float8, jsonb_to_csv(p.companies), jsonb_to_csv(p.investors),
		       p.amount::float8, p.currency, p.round_type, p.valuation::float8,
		       p.coverage_count, p.also_sources, p.ai_model_used,
		       to_char(p.published_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       to_char(p.fetched_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM processed_items p
		JOIN article_entities e ON e.article_id = p.id
		WHERE e.entity_name ILIKE $1 OR p.title ILIKE $1
		ORDER BY p.fi_score::numeric DESC NULLS LAST, p.published_at DESC NULLS LAST
		LIMIT $2
	`, pattern, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer articleRows.Close()

	articles := []processedItemOut{}
	for articleRows.Next() {
		item, err := scanProcessedItem(articleRows)
		if err == nil {
			articles = append(articles, item)
		}
	}

	// Sentiment + funding aggregate
	type sentStats struct {
		Bullish        int64   `json:"bullish"`
		Bearish        int64   `json:"bearish"`
		Neutral        int64   `json:"neutral"`
		AvgFiScore     float64 `json:"avgFiScore"`
		TotalFundingCr float64 `json:"totalFundingCr"`
		Total          int64   `json:"total"`
	}
	rate := fxrates.Global.GetUSDINR()
	var stats sentStats
	_ = h.pool.QueryRow(ctx, `
		SELECT
		    COUNT(CASE WHEN sentiment = 'bullish' THEN 1 END)::bigint,
		    COUNT(CASE WHEN sentiment = 'bearish' THEN 1 END)::bigint,
		    COUNT(CASE WHEN sentiment = 'neutral' THEN 1 END)::bigint,
		    COALESCE(AVG(fi_score::numeric), 0),
		    COALESCE(SUM(CASE 
		        WHEN amount IS NOT NULL AND currency = 'INR' THEN amount::numeric / 10000000 
		        WHEN amount IS NOT NULL AND currency = 'USD' THEN (amount::numeric * $2) / 10000000 
		        ELSE 0 
		    END), 0),
		    COUNT(*)::bigint
		FROM processed_items p
		LEFT JOIN article_entities e ON e.article_id = p.id
		WHERE e.entity_name ILIKE $1 OR p.title ILIKE $1
	`, pattern, rate).Scan(
		&stats.Bullish, &stats.Bearish, &stats.Neutral,
		&stats.AvgFiScore, &stats.TotalFundingCr, &stats.Total,
	)

	coRows, err := h.pool.Query(ctx, `
		SELECT
		    e2.entity_name as entity,
		    COUNT(*)::integer as co_count
		FROM article_entities e1
		JOIN article_entities e2 ON e1.article_id = e2.article_id
		WHERE e1.entity_name ILIKE $1 AND e2.entity_name NOT ILIKE $1
		GROUP BY e2.entity_name
		ORDER BY co_count DESC
		LIMIT 10
	`, pattern)

	topCoMentioned := []string{}
	if err == nil {
		defer coRows.Close()
		nameLower := strings.ToLower(name)
		for coRows.Next() {
			var entity string
			var cnt int
			if scanErr := coRows.Scan(&entity, &cnt); scanErr == nil {
				entity = strings.TrimSpace(entity)
				if entity != "" && len(entity) > 1 && strings.ToLower(entity) != nameLower {
					topCoMentioned = append(topCoMentioned, entity)
				}
			}
		}
		if len(topCoMentioned) > 8 {
			topCoMentioned = topCoMentioned[:8]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":           name,
		"articles":       articles,
		"totalMentions":  stats.Total,
		"avgFiScore":     math.Round(stats.AvgFiScore*10) / 10,
		"totalFundingCr": math.Round(stats.TotalFundingCr),
		"sentimentBreakdown": gin.H{
			"bullish": stats.Bullish,
			"bearish": stats.Bearish,
			"neutral": stats.Neutral,
		},
		"topCoMentioned": topCoMentioned,
	})
}
