package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
	"github.com/rs/zerolog/log"

	"github.com/financeintel/backend/internal/stealth"
)

// WorkerStat holds runtime metrics for a single RSS source.
type WorkerStat struct {
	Name         string
	Status       string // "idle" | "running" | "error"
	LastRun      *time.Time
	ItemsTotal   int64
	ErrorMessage string
}

// Worker fetches RSS from all configured sources and inserts raw_items into the DB.
type Worker struct {
	pool    *pgxpool.Pool
	fetcher *stealth.Fetcher

	mu    sync.RWMutex
	stats map[string]*WorkerStat

	StatusLastRun time.Time
	StatusItems   int64
}

// New creates an RSS worker. Call Run on a schedule.
func New(pool *pgxpool.Pool) *Worker {
	w := &Worker{
		pool:    pool,
		fetcher: stealth.New(),
		stats:   make(map[string]*WorkerStat, len(FeedSources)),
	}
	for _, src := range FeedSources {
		w.stats[src.Name] = &WorkerStat{Name: src.Name, Status: "idle"}
	}
	return w
}

// Run fetches all configured RSS sources in batches of 5.
// It is called by the scheduler on the RSS_INTERVAL_MINUTES cron.
func (w *Worker) Run(ctx context.Context) error {
	total := int64(0)
	concurrency := 10

	jobs := make(chan Source, len(FeedSources))
	var wg sync.WaitGroup

	// Start worker pool
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for src := range jobs {
				// Honor context cancellation
				if ctx.Err() != nil {
					return
				}
				n, err := w.fetchSource(ctx, src)
				if err != nil {
					log.Warn().Err(err).Str("source", src.Name).Msg("rss: fetch failed")
				}
				atomic.AddInt64(&total, int64(n))
			}
		}()
	}

	// Send jobs
	for _, src := range FeedSources {
		jobs <- src
	}
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	w.StatusLastRun = time.Now()
	atomic.AddInt64(&w.StatusItems, total)

	if total > 0 {
		log.Info().Int64("inserted", total).Msg("rss worker: cycle complete")
	}
	return nil
}

// RunSingle fetches a single source by name. Used by the manual trigger API.
func (w *Worker) RunSingle(ctx context.Context, sourceName string) (int, error) {
	for _, src := range FeedSources {
		if src.Name == sourceName {
			return w.fetchSource(ctx, src)
		}
	}
	return 0, fmt.Errorf("rss: source %q not found", sourceName)
}

// Stats returns a snapshot of all source stats for the /workers/status endpoint.
func (w *Worker) Stats() []WorkerStat {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]WorkerStat, 0, len(w.stats))
	for _, s := range w.stats {
		out = append(out, *s)
	}
	return out
}

// fetchSource fetches and parses one RSS feed, inserting new items into the DB.
func (w *Worker) fetchSource(ctx context.Context, src Source) (int, error) {
	w.setStat(src.Name, "running", "")

	// gofeed.Parser is NOT safe for concurrent use (it lazily initializes
	// shared translator state), and Run fetches 5 sources concurrently —
	// so each fetch gets its own parser instead of sharing one on Worker.
	parser := gofeed.NewParser()

	// Try stealth fetch first, fall back to gofeed direct parse
	var feed *gofeed.Feed
	result := w.fetcher.Fetch(ctx, src.URL, 12000)
	if result.OK && len(result.Body) > 100 {
		if f, err := parser.ParseString(result.Body); err == nil {
			feed = f
		}
	}

	if feed == nil {
		// ParseURL uses http.DefaultClient (no timeout) and
		// context.Background() internally — a hung origin would pin
		// this goroutine forever and ignore shutdown. Bound it with a
		// per-request timeout and honor the worker context.
		parser.Client = &http.Client{Timeout: 15 * time.Second}
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		f, err := parser.ParseURLWithContext(src.URL, reqCtx)
		cancel()
		if err != nil {
			w.setStat(src.Name, "error", err.Error())
			return 0, fmt.Errorf("fetch %s: %w", src.Name, err)
		}
		feed = f
	}

	inserted := 0
	for _, item := range feed.Items {
		if inserted >= 25 {
			break // cap per source per cycle
		}

		title := strings.TrimSpace(item.Title)
		url := item.Link
		if url == "" {
			url = item.GUID
		}
		snippet := stealth.StripHTML(coalesce(item.Description, item.Content))

		if title == "" || url == "" {
			continue
		}
		if !passesRelevanceFilter(title, snippet) {
			continue
		}

		hash := contentHash(title, url)
		var publishedAt *time.Time
		if item.PublishedParsed != nil {
			t := *item.PublishedParsed
			publishedAt = &t
		}

		tag, err := w.pool.Exec(ctx, `
                        INSERT INTO raw_items (source, source_type, url, title, snippet, author, published_at, content_hash, processed)
                        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE)
                        ON CONFLICT (url) DO NOTHING
                `, src.Name, "rss", url, title, nullIfEmpty(snippet), nullIfEmpty(authorName(item.Author)),
			publishedAt, hash)

		if err != nil {
			// Ignore unique constraint violations (duplicate URL) — they're expected
			var pgErr *pgconn.PgError
			isUniqueViolation := errors.As(err, &pgErr) && pgErr.Code == "23505"
			if !isUniqueViolation {
				log.Warn().Err(err).Str("url", url).Msg("rss: insert failed")
			}
			continue
		}
		// ON CONFLICT DO NOTHING returns no error but affects 0 rows on
		// a duplicate URL — only count genuinely new rows so the
		// per-source cap and reported totals reflect real inserts.
		if tag.RowsAffected() == 0 {
			continue
		}
		inserted++
	}

	now := time.Now()
	w.mu.Lock()
	if s, ok := w.stats[src.Name]; ok {
		s.Status = "idle"
		s.LastRun = &now
		s.ItemsTotal += int64(inserted)
		s.ErrorMessage = ""
	}
	w.mu.Unlock()

	return inserted, nil
}

// setStat updates the status of a source in the stats map.
func (w *Worker) setStat(name, status, errMsg string) {
	w.mu.Lock()
	if s, ok := w.stats[name]; ok {
		s.Status = status
		if errMsg != "" {
			s.ErrorMessage = errMsg
		}
	}
	w.mu.Unlock()
}

// contentHash produces a SHA256 fingerprint for (title, url) — Level 2 deduplication.
func contentHash(title, url string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(title)) + "::" + url))
	return hex.EncodeToString(h[:])[:32]
}

// passesRelevanceFilter returns true if the article is finance/startup relevant.
func passesRelevanceFilter(title, snippet string) bool {
	text := strings.ToLower(title + " " + snippet)
	for _, kw := range BroadKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// coalesce returns the first non-empty string from the arguments.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// nullIfEmpty returns nil for empty string (maps to SQL NULL).
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// authorName safely extracts the name from a *gofeed.Person (may be nil).
func authorName(p *gofeed.Person) string {
	if p == nil {
		return ""
	}
	return p.Name
}
