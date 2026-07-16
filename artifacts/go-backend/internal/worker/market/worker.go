package market

import (
        "context"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "strings"
        "sync"
        "sync/atomic"
        "time"

        "github.com/jackc/pgx/v5/pgxpool"
        "github.com/rs/zerolog/log"

        "github.com/financeintel/backend/internal/worker/rss"
)

// WSBroadcaster is the interface the market worker uses to send live ticks.
type WSBroadcaster interface {
        Broadcast(msgType string, payload interface{})
}

// Worker fetches market price data from Yahoo Finance and stores snapshots.
type Worker struct {
        pool          *pgxpool.Pool
        wsHub         WSBroadcaster
        httpClient    *http.Client
        StatusLastRun time.Time
        StatusUpdated int64
}

// New creates a market worker.
func New(pool *pgxpool.Pool, hub WSBroadcaster) *Worker {
        return &Worker{
                pool:  pool,
                wsHub: hub,
                httpClient: &http.Client{
                        Timeout: 15 * time.Second,
                },
        }
}

// yahooQuote holds the fields we care about from Yahoo Finance v8 API.
type yahooQuote struct {
        Symbol                      string  `json:"symbol"`
        ShortName                   string  `json:"shortName"`
        RegularMarketPrice          float64 `json:"regularMarketPrice"`
        RegularMarketChange         float64 `json:"regularMarketChange"`
        RegularMarketChangePercent  float64 `json:"regularMarketChangePercent"`
        RegularMarketPreviousClose  float64 `json:"regularMarketPreviousClose"`
}

// Run fetches prices for all market symbols and persists them as snapshots.
// It is called by the scheduler every MARKET_INTERVAL_MINUTES.
func (w *Worker) Run(ctx context.Context) error {
        symbols := rss.MarketSymbols

        // Batch symbols into groups of 10 — Yahoo Finance accepts multiple symbols per request
        batchSize := 10
        var snapshots []map[string]interface{}
        var mu sync.Mutex
        updated := int64(0)

        var wg sync.WaitGroup
        for i := 0; i < len(symbols); i += batchSize {
                end := i + batchSize
                if end > len(symbols) {
                        end = len(symbols)
                }
                batch := symbols[i:end]

                wg.Add(1)
                go func(batch []rss.MarketSymbol) {
                        defer wg.Done()

                        quotes, err := w.fetchYahooQuotes(ctx, batch)
                        if err != nil {
                                log.Warn().Err(err).Msg("market worker: yahoo fetch failed")
                                return
                        }

                        for _, q := range quotes {
                                if ctx.Err() != nil {
                                        return
                                }
                                if q.RegularMarketPrice == 0 {
                                        continue
                                }

                                // Find the matching symbol metadata
                                var sym rss.MarketSymbol
                                for _, s := range batch {
                                        if s.Symbol == q.Symbol {
                                                sym = s
                                                break
                                        }
                                }

                                _, err := w.pool.Exec(ctx, `
                                        INSERT INTO market_snapshots (symbol, name, exchange, price, change_pct, change_abs, prev_close)
                                        VALUES ($1, $2, $3, $4, $5, $6, $7)
                                `,
                                        q.Symbol,
                                        coalesce(sym.Name, q.ShortName),
                                        sym.Exchange,
                                        q.RegularMarketPrice,
                                        q.RegularMarketChangePercent,
                                        q.RegularMarketChange,
                                        q.RegularMarketPreviousClose,
                                )
                                if err != nil {
                                        log.Warn().Err(err).Str("symbol", q.Symbol).Msg("market worker: insert snapshot failed")
                                        continue
                                }

                                atomic.AddInt64(&updated, 1)

                                mu.Lock()
                                snapshots = append(snapshots, map[string]interface{}{
                                        "symbol":    q.Symbol,
                                        "name":      coalesce(sym.Name, q.ShortName),
                                        "price":     q.RegularMarketPrice,
                                        "changePct": q.RegularMarketChangePercent,
                                        "changeAbs": q.RegularMarketChange,
                                })
                                mu.Unlock()
                        }
                }(batch)
        }

        wg.Wait()

        if updated > 0 {
                log.Info().Int64("updated", updated).Msg("market worker: snapshots inserted")

                // Broadcast live tick to all WebSocket clients
                if w.wsHub != nil {
                        w.wsHub.Broadcast("MARKET_TICK", map[string]interface{}{
                                "snapshots": snapshots,
                                "updatedAt": time.Now().Format(time.RFC3339),
                        })
                }
        }

        w.StatusLastRun = time.Now()
        atomic.AddInt64(&w.StatusUpdated, updated)
        return nil
}

// fetchYahooQuotes calls the Yahoo Finance v8 API for a batch of symbols.
func (w *Worker) fetchYahooQuotes(ctx context.Context, symbols []rss.MarketSymbol) ([]yahooQuote, error) {
        syms := make([]string, len(symbols))
        for i, s := range symbols {
                syms[i] = s.Symbol
        }
        url := fmt.Sprintf(
                "https://query2.finance.yahoo.com/v8/finance/quote?symbols=%s&lang=en-US&region=US",
                strings.Join(syms, ","),
        )

        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
                return nil, err
        }
        req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
        req.Header.Set("Accept", "application/json")

        resp, err := w.httpClient.Do(req)
        if err != nil {
                return nil, fmt.Errorf("yahoo finance: %w", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
                body, _ := io.ReadAll(resp.Body)
                return nil, fmt.Errorf("yahoo finance: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 200)])))
        }

        body, _ := io.ReadAll(resp.Body)

        var response struct {
                QuoteResponse struct {
                        Result []yahooQuote `json:"result"`
                } `json:"quoteResponse"`
        }
        if err := json.Unmarshal(body, &response); err != nil {
                return nil, fmt.Errorf("yahoo finance parse: %w", err)
        }

        return response.QuoteResponse.Result, nil
}

func coalesce(vals ...string) string {
        for _, v := range vals {
                if v != "" {
                        return v
                }
        }
        return ""
}
