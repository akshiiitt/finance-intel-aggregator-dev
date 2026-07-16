package handlers

import (
        "net/http"
        "time"

        "github.com/gin-gonic/gin"
        "github.com/jackc/pgx/v5/pgxpool"
)

// MarketHandler handles all /api/market/* routes.
type MarketHandler struct {
        pool *pgxpool.Pool
}

// NewMarketHandler creates a market handler.
func NewMarketHandler(pool *pgxpool.Pool) *MarketHandler {
        return &MarketHandler{pool: pool}
}

// marketSnapshotOut is the flat shape returned for each instrument.
// Matches the Node.js marketSnapshot shape exactly:
//
//      { symbol, name, exchange, region, price, changePct, changeAbs, prevClose, updatedAt }
type marketSnapshotOut struct {
        Symbol    string   `json:"symbol"`
        Name      *string  `json:"name"`
        Exchange  string   `json:"exchange"`
        Region    string   `json:"region"`
        Price     float64  `json:"price"`
        ChangePct *float64 `json:"changePct"`
        ChangeAbs *float64 `json:"changeAbs"`
        PrevClose *float64 `json:"prevClose"`
        UpdatedAt string   `json:"updatedAt"`
}

// regionFromExchange derives the region bucket from the exchange string.
// Matches the Node.js tagging logic.
func regionFromExchange(exchange string) string {
        switch exchange {
        case "NSE", "BSE":
                return "india"
        case "FX":
                return "fx"
        case "CRYPTO":
                return "crypto"
        default:
                return "global"
        }
}

const snapshotCols = `
        symbol, name, exchange,
        price::float8, change_pct::float8, change_abs::float8, prev_close::float8,
        to_char(captured_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS captured_at`

// scanSnapshot scans one row into marketSnapshotOut.
func scanSnapshot(rows interface{ Scan(...any) error }) (marketSnapshotOut, error) {
        var s marketSnapshotOut
        err := rows.Scan(
                &s.Symbol, &s.Name, &s.Exchange,
                &s.Price, &s.ChangePct, &s.ChangeAbs, &s.PrevClose, &s.UpdatedAt,
        )
        s.Region = regionFromExchange(s.Exchange)
        return s, err
}

// GetMarket handles GET /api/market
// Returns the latest snapshot for each tracked symbol as a flat list.
// Response shape mirrors Node.js: { snapshots: [...], updatedAt: "<ISO>" }
func (h *MarketHandler) GetMarket(c *gin.Context) {
        rows, err := h.pool.Query(c.Request.Context(), `
                SELECT DISTINCT ON (symbol) `+snapshotCols+`
                FROM market_snapshots
                ORDER BY symbol, captured_at DESC
        `)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
                return
        }
        defer rows.Close()

        snapshots := []marketSnapshotOut{}
        latestAt := ""

        for rows.Next() {
                s, err := scanSnapshot(rows)
                if err != nil {
                        continue
                }
                snapshots = append(snapshots, s)
                if s.UpdatedAt > latestAt {
                        latestAt = s.UpdatedAt
                }
        }

        if latestAt == "" {
                latestAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
        }

        c.JSON(http.StatusOK, gin.H{
                "snapshots": snapshots,
                "updatedAt": latestAt,
        })
}

// GetSymbolHistory handles GET /api/market/history
// Query params: symbol (optional — omit for all symbols), hours (default 24, max 168 = 7 days)
// Response shape mirrors Node.js exactly: { history: [...] }
func (h *MarketHandler) GetSymbolHistory(c *gin.Context) {
        symbol := c.Query("symbol")

        hours := floatQuery(c, "hours", 24)
        if hours > 168 {
                hours = 168 // max 7 days — matches Node.js
        }
        since := time.Now().Add(-time.Duration(hours) * time.Hour)

        var rows interface {
                Next() bool
                Scan(...any) error
                Close()
        }

        var err error
        if symbol != "" {
                rows, err = h.pool.Query(c.Request.Context(), `
                        SELECT `+snapshotCols+`
                        FROM market_snapshots
                        WHERE symbol = $1 AND captured_at >= $2
                        ORDER BY symbol, captured_at ASC
                        LIMIT 2000
                `, symbol, since)
        } else {
                rows, err = h.pool.Query(c.Request.Context(), `
                        SELECT `+snapshotCols+`
                        FROM market_snapshots
                        WHERE captured_at >= $1
                        ORDER BY symbol, captured_at ASC
                        LIMIT 5000
                `, since)
        }
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
                return
        }
        defer rows.Close()

        history := []marketSnapshotOut{}
        for rows.Next() {
                s, err := scanSnapshot(rows)
                if err == nil {
                        history = append(history, s)
                }
        }

        // Wrap in { history: [...] } — matches Node.js response shape
        c.JSON(http.StatusOK, gin.H{"history": history})
}
