package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// IPOHandler handles all /api/ipo/* routes.
type IPOHandler struct {
	pool *pgxpool.Pool
}

// NewIPOHandler creates an IPO handler.
func NewIPOHandler(pool *pgxpool.Pool) *IPOHandler {
	return &IPOHandler{pool: pool}
}

type ipoRow struct {
	ID            int64    `json:"id"`
	CompanyName   string   `json:"companyName"`
	Exchange      *string  `json:"exchange"`
	PriceBandLow  *float64 `json:"priceBandLow"`
	PriceBandHigh *float64 `json:"priceBandHigh"`
	LotSize       *int     `json:"lotSize"`
	OpenDate      *string  `json:"openDate"`
	CloseDate     *string  `json:"closeDate"`
	ListingDate   *string  `json:"listingDate"`
	IssueSizeCr   *float64 `json:"issueSizeCr"`
	GMP           *float64 `json:"gmp"`
	SubscriptionX *float64 `json:"subscriptionX"`
	Status        *string  `json:"status"`
	Sector        *string  `json:"sector"`
	UpdatedAt     string   `json:"updatedAt"`
}

const ipoSelectCols = `
	id, company_name, exchange,
	price_band_low::float8, price_band_high::float8,
	lot_size,
	to_char(open_date,'YYYY-MM-DD'),
	to_char(close_date,'YYYY-MM-DD'),
	to_char(listing_date,'YYYY-MM-DD'),
	issue_size_cr::float8, gmp::float8, subscription_x::float8,
	status, sector,
	to_char(updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
`

func scanIPORow(rows interface{ Scan(...any) error }) (ipoRow, error) {
	var r ipoRow
	err := rows.Scan(
		&r.ID, &r.CompanyName, &r.Exchange,
		&r.PriceBandLow, &r.PriceBandHigh,
		&r.LotSize,
		&r.OpenDate, &r.CloseDate, &r.ListingDate,
		&r.IssueSizeCr, &r.GMP, &r.SubscriptionX,
		&r.Status, &r.Sector, &r.UpdatedAt,
	)
	return r, err
}

// GetIPOs handles GET /api/ipo
// Returns IPOs grouped by status, mirroring the Node.js response shape exactly:
//
//	{ upcoming: [...], open: [...], closed: [...], listed: [...] }
//
// Optional query param ?status= returns a flat array for that status only
// (used by the dashboard filter panel).
func (h *IPOHandler) GetIPOs(c *gin.Context) {
	statusFilter := c.Query("status")
	limit := intQuery(c, "limit", 200)
	if limit > 500 {
		limit = 500
	}
	offset := intQuery(c, "offset", 0)

	// When a specific status is requested, return a flat filtered list
	// (matches Node.js behaviour when the dashboard filters)
	if statusFilter != "" && statusFilter != "all" {
		rows, err := h.pool.Query(c.Request.Context(), `
			SELECT `+ipoSelectCols+`
			FROM ipo_calendar
			WHERE status = $1
			ORDER BY open_date DESC NULLS LAST, updated_at DESC
			LIMIT $2 OFFSET $3
		`, statusFilter, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}
		defer rows.Close()
		result := []ipoRow{}
		for rows.Next() {
			if r, err := scanIPORow(rows); err == nil {
				result = append(result, r)
			}
		}
		c.JSON(http.StatusOK, result)
		return
	}

	// Default: return grouped by status — matches Node.js main endpoint
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT `+ipoSelectCols+`
		FROM ipo_calendar
		ORDER BY
		    CASE status
		        WHEN 'open'     THEN 1
		        WHEN 'upcoming' THEN 2
		        WHEN 'closed'   THEN 3
		        WHEN 'listed'   THEN 4
		        ELSE 5
		    END,
		    open_date DESC NULLS LAST
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	grouped := map[string][]ipoRow{
		"upcoming": {},
		"open":     {},
		"closed":   {},
		"listed":   {},
	}

	for rows.Next() {
		r, err := scanIPORow(rows)
		if err != nil {
			continue
		}
		status := "upcoming"
		if r.Status != nil {
			status = *r.Status
		}
		if _, ok := grouped[status]; ok {
			grouped[status] = append(grouped[status], r)
		} else {
			grouped["upcoming"] = append(grouped["upcoming"], r)
		}
	}

	c.JSON(http.StatusOK, grouped)
}

// CreateIPO handles POST /api/ipo
func (h *IPOHandler) CreateIPO(c *gin.Context) {
	var body struct {
		CompanyName   string   `json:"companyName" binding:"required"`
		Exchange      *string  `json:"exchange"`
		PriceBandLow  *float64 `json:"priceBandLow"`
		PriceBandHigh *float64 `json:"priceBandHigh"`
		LotSize       *int     `json:"lotSize"`
		OpenDate      *string  `json:"openDate"`
		CloseDate     *string  `json:"closeDate"`
		ListingDate   *string  `json:"listingDate"`
		IssueSizeCr   *float64 `json:"issueSizeCr"`
		GMP           *float64 `json:"gmp"`
		SubscriptionX *float64 `json:"subscriptionX"`
		Status        *string  `json:"status"`
		Sector        *string  `json:"sector"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := "upcoming"
	if body.Status != nil {
		status = *body.Status
	}

	var id int64
	// company_name is UNIQUE, and the IPO worker re-scrapes the same companies
	// on every cycle — a bare INSERT would throw a 23505 on the second sighting.
	// Upsert so re-scrapes refresh the volatile fields (GMP, subscription,
	// status, dates) instead of failing.
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO ipo_calendar (
		    company_name, exchange, price_band_low, price_band_high, lot_size,
		    open_date, close_date, listing_date, issue_size_cr, gmp, subscription_x, status, sector
		) VALUES (
		    $1, $2, $3, $4, $5,
		    $6::date, $7::date, $8::date, $9, $10, $11, $12, $13
		)
		ON CONFLICT (company_name) DO UPDATE SET
		    exchange        = COALESCE(EXCLUDED.exchange, ipo_calendar.exchange),
		    price_band_low  = COALESCE(EXCLUDED.price_band_low, ipo_calendar.price_band_low),
		    price_band_high = COALESCE(EXCLUDED.price_band_high, ipo_calendar.price_band_high),
		    lot_size        = COALESCE(EXCLUDED.lot_size, ipo_calendar.lot_size),
		    open_date       = COALESCE(EXCLUDED.open_date, ipo_calendar.open_date),
		    close_date      = COALESCE(EXCLUDED.close_date, ipo_calendar.close_date),
		    listing_date    = COALESCE(EXCLUDED.listing_date, ipo_calendar.listing_date),
		    issue_size_cr   = COALESCE(EXCLUDED.issue_size_cr, ipo_calendar.issue_size_cr),
		    gmp             = COALESCE(EXCLUDED.gmp, ipo_calendar.gmp),
		    subscription_x  = COALESCE(EXCLUDED.subscription_x, ipo_calendar.subscription_x),
		    status          = EXCLUDED.status,
		    sector          = COALESCE(EXCLUDED.sector, ipo_calendar.sector),
		    updated_at      = NOW()
		RETURNING id
	`, body.CompanyName, body.Exchange, body.PriceBandLow, body.PriceBandHigh, body.LotSize,
		body.OpenDate, body.CloseDate, body.ListingDate, body.IssueSizeCr,
		body.GMP, body.SubscriptionX, status, body.Sector,
	).Scan(&id)
	if err != nil {
		log.Error().Err(err).Msg("ipo: insert failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "IPO created"})
}

// UpdateIPO handles PATCH /api/ipo/:id — partial updates only.
func (h *IPOHandler) UpdateIPO(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowed := map[string]string{
		"companyName":   "company_name",
		"exchange":      "exchange",
		"priceBandLow":  "price_band_low",
		"priceBandHigh": "price_band_high",
		"lotSize":       "lot_size",
		"openDate":      "open_date",
		"closeDate":     "close_date",
		"listingDate":   "listing_date",
		"issueSizeCr":   "issue_size_cr",
		"gmp":           "gmp",
		"subscriptionX": "subscription_x",
		"status":        "status",
		"sector":        "sector",
	}

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{id}
	argN := 2

	for jsonKey, colName := range allowed {
		if val, ok := body[jsonKey]; ok {
			if (colName == "status" || colName == "company_name") && val == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": colName + " cannot be null"})
				return
			}
			setClauses = append(setClauses, colName+" = $"+strconv.Itoa(argN))
			args = append(args, val)
			argN++
		}
	}

	if len(setClauses) == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	_, err = h.pool.Exec(c.Request.Context(),
		"UPDATE ipo_calendar SET "+joinComma(setClauses)+" WHERE id = $1", args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// DeleteIPO handles DELETE /api/ipo/:id
func (h *IPOHandler) DeleteIPO(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, err = h.pool.Exec(c.Request.Context(), `DELETE FROM ipo_calendar WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
