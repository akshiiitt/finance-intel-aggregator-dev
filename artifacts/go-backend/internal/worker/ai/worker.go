package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/financeintel/backend/internal/embed"
	"github.com/financeintel/backend/internal/fxrates"
	"github.com/financeintel/backend/internal/scorer"
)

// batchItem is a lightweight struct passed to AI models / the embedding
// sidecar for a single article.
type batchItem struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

// WSBroadcaster is the minimal interface any worker needs from the
// WebSocket hub to push a live update.
type WSBroadcaster interface {
	Broadcast(msgType string, payload any)
}

// Worker is the FREE processing pass. For every unprocessed raw_item it:
//  1. embeds the title+snippet via the local embedding sidecar (free, no API)
//  2. dedups against the last 48h via pgvector cosine distance (falls back
//     to legacy Jaccard title matching if the sidecar is unreachable)
//  3. classifies with keywordProcess — zero-cost regex rules, no network call
//  4. tags niches from the same embedding (reused, no extra cost)
//  5. computes the FIScore
//  6. flags ai_pending for the small slice worth a paid API call later
//
// It never calls Groq or Gemini. That's EnrichWorker's job (enrich.go), run
// on its own schedule against only the rows this worker flags — so the feed
// is fully functional and ranked even if the paid AI budget is exhausted or
// the provider is down.
type Worker struct {
	pool           *pgxpool.Pool
	embed          *embed.Client
	batchSize      int
	dedupThreshold float64 // cosine SIMILARITY threshold (e.g. 0.88)
	aiPendingMinFi float64 // non-deal articles need at least this FIScore to be flagged for a paid summary
	wsHub          WSBroadcaster
	running        int32 // CAS guard — see Run()
	StatusLastRun  time.Time
	StatusItems    int64 // updated via atomic.AddInt64
}

// New creates the free processing worker. It warms the niche anchors and
// the legacy Jaccard title cache (fallback dedup) in the background so
// startup stays fast.
func New(pool *pgxpool.Pool, ec *embed.Client, batchSize int, dedupThreshold, aiPendingMinFi float64, hub WSBroadcaster) *Worker {
	w := &Worker{
		pool:           pool,
		embed:          ec,
		batchSize:      batchSize,
		dedupThreshold: dedupThreshold,
		aiPendingMinFi: aiPendingMinFi,
		wsHub:          hub,
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if ec.Healthy(ctx) {
			InitNicheAnchors(ctx, ec)
		} else {
			log.Warn().Msg("ai worker: embed sidecar not reachable at startup — dedup/niches degrade to keyword-only until it recovers")
		}

		// Legacy Jaccard cache stays warm as the fallback dedup path for any
		// cycle where the embedding sidecar happens to be down.
		InitTitleCache(ctx, pool)
	}()

	return w
}

// Run processes one batch of unprocessed raw items — the free pass only.
// It is called by the scheduler every N seconds, and can also be invoked
// on-demand via POST /api/workers/ai/trigger. Those two callers used to be
// able to run concurrently against the same unprocessed-item query (no row
// locking), causing duplicate processing. The CAS guard below makes a
// second concurrent call a safe, logged no-op instead — simpler than
// SELECT ... FOR UPDATE SKIP LOCKED and sufficient since this is a
// single-process worker (see docs on why this isn't horizontally scaled).
func (w *Worker) Run(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		log.Debug().Msg("ai worker: skipped — a free-pass run is already in progress")
		return nil
	}
	defer atomic.StoreInt32(&w.running, 0)
	return w.run(ctx)
}

func (w *Worker) run(ctx context.Context) error {
	usdINR := fxrates.Global.GetUSDINR()

	rows, err := w.pool.Query(ctx, `
		SELECT id, title, COALESCE(snippet, '') as snippet, source,
		       COALESCE(source_type, 'rss') as source_type, url as source_url, published_at
		FROM raw_items
		WHERE processed = FALSE AND processing_error IS NULL
		ORDER BY fetched_at ASC
		LIMIT $1
	`, w.batchSize)
	if err != nil {
		return fmt.Errorf("ai worker: query raw items: %w", err)
	}

	type rawItem struct {
		ID          int64
		Title       string
		Snippet     string
		Source      string
		SourceType  string
		SourceURL   string
		PublishedAt *time.Time
	}

	var items []rawItem
	for rows.Next() {
		var ri rawItem
		if err := rows.Scan(&ri.ID, &ri.Title, &ri.Snippet, &ri.Source, &ri.SourceType, &ri.SourceURL, &ri.PublishedAt); err != nil {
			continue
		}
		items = append(items, ri)
	}
	rows.Close()

	if len(items) == 0 {
		return nil
	}

	if len(items) == w.batchSize {
		var remaining int64
		_ = w.pool.QueryRow(ctx, `SELECT COUNT(*) FROM raw_items WHERE processed = FALSE AND processing_error IS NULL`).Scan(&remaining)
		if remaining > 0 {
			log.Warn().Int64("remaining", remaining).Msg("ai worker: batch size limit reached; unprocessed items still remaining in queue")
		}
	}

	// Embed the whole batch in one call. If the sidecar is down, vectors is
	// nil for every item and we fall back to the legacy Jaccard dedup with
	// no niche tagging this cycle — degrade, never break.
	texts := make([]string, len(items))
	for i, it := range items {
		texts[i] = it.Title + ". " + truncate(it.Snippet, 300)
	}
	vectors, embedErr := w.embed.Embed(ctx, texts)
	if embedErr != nil {
		log.Warn().Err(embedErr).Msg("ai worker: embedding sidecar unavailable this cycle, falling back to Jaccard dedup + no niches")
		vectors = nil
	}

	processed := int64(0)

	for i, it := range items {
		var vec []float32
		if vectors != nil {
			vec = vectors[i]
		}

		// ── Dedup: vector cosine (preferred) or Jaccard fallback ─────────────
		existingID, existingSrc, isDup := w.checkDuplicate(ctx, vec, it.Title, it.SourceURL)

		// ── Free classification — always runs, zero network cost ────────────
		merged := keywordProcess(it.Title, it.Snippet)

		if isDup {
			var currentCoverage int
			_ = w.pool.QueryRow(ctx, `SELECT coverage_count FROM processed_items WHERE id = $1`, existingID).Scan(&currentCoverage)
			if currentCoverage <= 0 {
				currentCoverage = 1
			}

			scoreInput := scorer.Input{
				RelevanceScore: merged.RelevanceScore,
				PublishedAt:    it.PublishedAt,
				Source:         it.Source,
				Amount:         merged.Amount,
				Currency:       merged.Currency,
				Category:       merged.Category,
				SentimentScore: merged.SentimentScore,
				KeyPointCount:  len(merged.KeyPoints),
				CoverageCount:  currentCoverage + 1,
				USDINRRate:     usdINR,
			}
			newFiScore := scorer.Calculate(scoreInput)

			tx, txErr := w.pool.Begin(ctx)
			if txErr != nil {
				log.Warn().Err(txErr).Msg("ai worker: begin tx failed for duplicate update")
				continue
			}
			_, updateErr := tx.Exec(ctx, `
				UPDATE processed_items
				SET coverage_count = coverage_count + 1,
				    also_sources = CASE
				        WHEN also_sources IS NULL THEN $2
				        WHEN also_sources NOT LIKE '%' || $2 || '%' THEN also_sources || ', ' || $2
				        ELSE also_sources
				    END,
				    fi_score = GREATEST(fi_score, $3::numeric)
				WHERE id = $1
			`, existingID, it.Source, newFiScore)
			if updateErr != nil {
				_ = tx.Rollback(ctx)
				log.Warn().Err(updateErr).Msg("ai worker: duplicate update failed")
				continue
			}
			_, updateRawErr := tx.Exec(ctx, `UPDATE raw_items SET processed = TRUE WHERE id = $1`, it.ID)
			if updateRawErr != nil {
				_ = tx.Rollback(ctx)
				log.Warn().Err(updateRawErr).Msg("ai worker: duplicate raw_items update failed")
				continue
			}
			_ = tx.Commit(ctx)

			log.Debug().
				Int64("existingID", existingID).
				Str("source", it.Source).
				Str("existingSource", existingSrc).
				Str("title", truncate(it.Title, 60)).
				Msg("ai worker: duplicate detected, coverage_count incremented")
			processed++
			continue
		}

		niches := TagNiches(vec)

		scoreInput := scorer.Input{
			RelevanceScore: merged.RelevanceScore,
			PublishedAt:    it.PublishedAt,
			Source:         it.Source,
			Amount:         merged.Amount,
			Currency:       merged.Currency,
			Category:       merged.Category,
			SentimentScore: merged.SentimentScore,
			KeyPointCount:  len(merged.KeyPoints),
			CoverageCount:  1,
			USDINRRate:     usdINR,
		}
		fiScore := scorer.Calculate(scoreInput)

		// companies/investors are stored as real JSONB arrays (see migration
		// 000004) — always write a valid array, never NULL, so containment
		// queries and jsonb_array_elements_text never have to special-case it.
		companiesJSON := toJSONArray(merged.Companies)
		investorsJSON := toJSONArray(merged.Investors)

		keyPointsJSON := "[]"
		if len(merged.KeyPoints) > 0 {
			if b, err := json.Marshal(merged.KeyPoints); err == nil {
				keyPointsJSON = string(b)
			}
		}

		// ── AI gating: is this worth a paid API call later? ─────────────────
		// Deal-type stories always get queued for entity extraction; anything
		// else only gets queued for a paid summary if it scored high enough
		// to actually be feed-worthy. Everything else stays exactly as the
		// free pass produced it, forever — that's expected, not a fallback.
		aiPending := isDealCategory(merged.Category) || fiScore >= w.aiPendingMinFi

		var embeddingSQL any
		if vec != nil {
			embeddingSQL = embed.ToSQL(vec)
		}

		var newID int64
		
		// Use a transaction to safely insert the article and populate the junction table
		tx, txErr := w.pool.Begin(ctx)
		if txErr != nil {
			log.Warn().Err(txErr).Msg("ai worker: begin tx failed")
			continue
		}

		insertErr := tx.QueryRow(ctx, `
			INSERT INTO processed_items (
			    raw_item_id, title, summary, key_points, source_url, source, source_type,
			    region, category, sentiment, sentiment_score, relevance_score, fi_score,
			    companies, investors, amount, currency, round_type, valuation,
			    ai_model_used, published_at, embedding, niches, ai_pending
			) VALUES (
			    $1, $2, $3, $4, $5, $6, $7,
			    $8, $9, $10, $11, $12, $13,
			    $14::jsonb, $15::jsonb, $16, $17, $18, $19,
			    $20, $21, $22::vector, $23, $24
			)
			RETURNING id
		`,
			it.ID, it.Title, merged.Summary, keyPointsJSON, it.SourceURL, it.Source, it.SourceType,
			merged.Region, merged.Category, merged.Sentiment, merged.SentimentScore, merged.RelevanceScore, fiScore,
			companiesJSON, investorsJSON, merged.Amount, nullIfEmpty(merged.Currency),
			nullIfEmpty(merged.RoundType), merged.Valuation,
			merged.AIModel, it.PublishedAt, embeddingSQL, niches, aiPending,
		).Scan(&newID)
		if insertErr != nil {
			_ = tx.Rollback(ctx)
			_, _ = w.pool.Exec(ctx, `UPDATE raw_items SET processing_error = $1 WHERE id = $2`,
				truncate(insertErr.Error(), 120), it.ID)
			log.Warn().Err(insertErr).Int64("id", it.ID).Msg("ai worker: insert processed item failed")
			continue
		}

		// Insert into junction table
		for _, co := range merged.Companies {
			if co != "" {
				_, _ = tx.Exec(ctx, `INSERT INTO article_entities (article_id, entity_name, entity_type) VALUES ($1, $2, 'company') ON CONFLICT DO NOTHING`, newID, co)
			}
		}
		for _, inv := range merged.Investors {
			if inv != "" {
				_, _ = tx.Exec(ctx, `INSERT INTO article_entities (article_id, entity_name, entity_type) VALUES ($1, $2, 'investor') ON CONFLICT DO NOTHING`, newID, inv)
			}
		}
		
		_, _ = tx.Exec(ctx, `UPDATE raw_items SET processed = TRUE WHERE id = $1`, it.ID)
		
		_ = tx.Commit(ctx)

		// Keep the legacy title cache warm too, so dedup still works on any
		// cycle where the embedding sidecar happens to be unreachable.
		RegisterTitle(it.Title, it.SourceURL, newID, it.Source)

		processed++

		if w.wsHub != nil {
			w.wsHub.Broadcast("ARTICLE_ENGAGED", map[string]any{
				"id":        newID,
				"title":     it.Title,
				"summary":   merged.Summary,
				"fiScore":   fiScore,
				"category":  merged.Category,
				"source":    it.Source,
				"region":    merged.Region,
				"sentiment": merged.Sentiment,
				"aiModel":   merged.AIModel,
				"niches":    niches,
			})
		}
	}

	w.StatusLastRun = time.Now()
	atomic.AddInt64(&w.StatusItems, processed)

	if processed > 0 {
		log.Info().Int64("processed", processed).Int("total", len(items)).Msg("ai worker: batch done (free pass)")
	}

	return nil
}

// checkDuplicate finds a near-duplicate of (title, vec) among recently
// processed items. It prefers pgvector cosine distance when an embedding is
// available (catches paraphrased headlines the old Jaccard check missed —
// e.g. "Zepto raises $102M" vs "Tiger Global leads Zepto's Series F"), and
// falls back to legacy Jaccard title matching when it isn't.
func (w *Worker) checkDuplicate(ctx context.Context, vec []float32, title, url string) (existingID int64, existingSource string, found bool) {
	// Exact URL match short-circuits either path — cheapest, most certain.
	if url != "" {
		var id int64
		var src string
		if err := w.pool.QueryRow(ctx, `SELECT id, source FROM processed_items WHERE source_url = $1 LIMIT 1`, url).Scan(&id, &src); err == nil {
			return id, src, true
		}
	}

	if vec != nil {
		id, src, isDup, err := w.checkDuplicateVector(ctx, vec)
		if err != nil {
			log.Warn().Err(err).Msg("ai worker: vector dedup query failed, falling back to Jaccard for this item")
		} else {
			if isDup {
				return id, src, true
			}
			// A clean "no vector match" result is trustworthy on its own —
			// don't also run Jaccard, which would just add noise.
			return 0, "", false
		}
	}

	return CheckDuplicate(title, url)
}

// checkDuplicateVector runs the cosine-distance nearest-neighbor query
// against the hot (48h) window. Distance = 1 - cosine similarity, so a
// similarity threshold of e.g. 0.88 becomes a distance threshold of 0.12.
func (w *Worker) checkDuplicateVector(ctx context.Context, vec []float32) (existingID int64, existingSource string, found bool, err error) {
	maxDistance := 1.0 - w.dedupThreshold

	var id int64
	var src string
	var distance float64
	row := w.pool.QueryRow(ctx, `
		SELECT id, source, embedding <=> $1::vector AS distance
		FROM processed_items
		WHERE embedding IS NOT NULL
		  AND fetched_at > now() - interval '48 hours'
		ORDER BY embedding <=> $1::vector
		LIMIT 1
	`, embed.ToSQL(vec))

	if err := row.Scan(&id, &src, &distance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", false, nil
		}
		return 0, "", false, err
	}
	if distance <= maxDistance {
		return id, src, true, nil
	}
	return 0, "", false, nil
}

// isDealCategory reports whether a category is one where entity extraction
// (companies, investors, valuation) actually adds value — everything else
// is already fully legible from its headline and free summary.
func isDealCategory(cat string) bool {
	switch cat {
	case "funding", "ipo", "mergers":
		return true
	default:
		return false
	}
}

// nullIfEmpty returns nil for an empty string (maps to SQL NULL).
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// toJSONArray marshals a string slice to a JSON array literal, treating nil
// the same as empty — companies/investors are NOT NULL JSONB columns (see
// migration 000004), so every write must produce a valid array, never SQL
// NULL and never the Go zero value "null".
func toJSONArray(items []string) string {
	if items == nil {
		items = []string{}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}
