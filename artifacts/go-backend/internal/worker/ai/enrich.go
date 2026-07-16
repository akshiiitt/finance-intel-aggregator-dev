package ai

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/financeintel/backend/internal/broker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// QuotaState tracks daily API call counts per AI provider. All fields are
// guarded by mu. Counts reset at midnight automatically. This lives on
// EnrichWorker because it's the ONLY worker that spends paid API quota —
// the free Process worker (worker.go) never touches Groq or Gemini.
type QuotaState struct {
	mu           sync.Mutex
	GeminiCalls  int64
	Groq8bCalls  int64
	Groq70bCalls int64
	date         string
}

const (
	geminiDailyCap  = 1400 // stay under Google's 1,500/day free ceiling
	groq70bDailyCap = 900
)

// pendingItem is one row the free Process worker flagged ai_pending = TRUE.
type pendingItem struct {
	ID       int64
	Title    string
	Snippet  string
	Category string
	FiScore  float64
}

// EnrichWorker is the PAID pass. It only ever looks at rows the free Process
// worker already flagged ai_pending = TRUE — by design that's a small slice
// of the feed (deal-type stories, or anything that scored high enough to be
// feed-worthy). Everything else in the feed was never touched by a paid API.
//
// Deal-type stories (funding/ipo/mergers) get full entity extraction
// (companies, investors, valuation) plus a written summary. Everything else
// that made the cut gets a summary only — a markets/policy headline doesn't
// need "companies mentioned," it needs a readable gloss.
type EnrichWorker struct {
	pool          *pgxpool.Pool
	groqKey       string
	geminiKey     string
	batchSize     int
	quota         *QuotaState
	wsHub         WSBroadcaster
	running       int32 // CAS guard — see Run()
	StatusLastRun time.Time
	StatusItems   int64
}

// NewEnrichWorker creates the paid enrichment worker.
func NewEnrichWorker(pool *pgxpool.Pool, groqKey, geminiKey string, batchSize int, hub WSBroadcaster) *EnrichWorker {
	return &EnrichWorker{
		pool:      pool,
		groqKey:   groqKey,
		geminiKey: geminiKey,
		batchSize: batchSize,
		quota:     &QuotaState{date: time.Now().UTC().Format("2006-01-02")},
		wsHub:     hub,
	}
}

// GetQuota returns a snapshot of the current daily quota state. Also
// implements handlers.QuotaProvider for GET /api/feed/stats.
func (w *EnrichWorker) GetQuota() (gemini, groq8b, groq70b int64) {
	w.quota.mu.Lock()
	defer w.quota.mu.Unlock()
	return w.quota.GeminiCalls, w.quota.Groq8bCalls, w.quota.Groq70bCalls
}

func (w *EnrichWorker) loadQuota(ctx context.Context) {
	today := time.Now().UTC().Format("2006-01-02")
	var gemini, groq8b, groq70b int64
	err := w.pool.QueryRow(ctx, `
		SELECT gemini_calls, groq8b_calls, groq70b_calls
		FROM ai_quotas WHERE quota_date = $1
	`, today).Scan(&gemini, &groq8b, &groq70b)

	w.quota.mu.Lock()
	defer w.quota.mu.Unlock()
	w.quota.date = today
	if err == nil {
		w.quota.GeminiCalls, w.quota.Groq8bCalls, w.quota.Groq70bCalls = gemini, groq8b, groq70b
	} else {
		w.quota.GeminiCalls, w.quota.Groq8bCalls, w.quota.Groq70bCalls = 0, 0, 0
	}
}

func (w *EnrichWorker) addQuota(ctx context.Context, gemini, groq8b, groq70b int64) {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := w.pool.Exec(ctx, `
		INSERT INTO ai_quotas (quota_date, gemini_calls, groq8b_calls, groq70b_calls, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (quota_date) DO UPDATE
		SET gemini_calls = ai_quotas.gemini_calls + EXCLUDED.gemini_calls,
		    groq8b_calls = ai_quotas.groq8b_calls + EXCLUDED.groq8b_calls,
		    groq70b_calls = ai_quotas.groq70b_calls + EXCLUDED.groq70b_calls,
		    updated_at = NOW()
	`, today, gemini, groq8b, groq70b)
	if err != nil {
		log.Error().Err(err).Msg("enrich worker: failed to update persistent quota")
	}

	w.quota.mu.Lock()
	if w.quota.date != today {
		w.quota.date = today
		w.quota.GeminiCalls, w.quota.Groq8bCalls, w.quota.Groq70bCalls = 0, 0, 0
	}
	w.quota.GeminiCalls += gemini
	w.quota.Groq8bCalls += groq8b
	w.quota.Groq70bCalls += groq70b
	w.quota.mu.Unlock()
}

func (w *EnrichWorker) addGemini(ctx context.Context, n int64)  { w.addQuota(ctx, n, 0, 0) }
func (w *EnrichWorker) addGroq70b(ctx context.Context, n int64) { w.addQuota(ctx, 0, 0, n) }

// Run picks up to batchSize ai_pending rows, richest-first, and spends the
// paid AI budget on them. It is called by the scheduler every N seconds and
// can also be invoked on-demand via POST /api/workers/enrich/trigger — the
// CAS guard keeps those two callers from ever spending API budget on the
// same rows concurrently.
func (w *EnrichWorker) Run(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		log.Debug().Msg("enrich worker: skipped — a paid-pass run is already in progress")
		return nil
	}
	defer atomic.StoreInt32(&w.running, 0)
	return w.run(ctx)
}

func (w *EnrichWorker) run(ctx context.Context) error {
	w.loadQuota(ctx)

	rows, err := w.pool.Query(ctx, `
		SELECT id, title, COALESCE(summary, '') as summary, COALESCE(category, '') as category,
		       COALESCE(fi_score, 0)::float8 as fi_score
		FROM processed_items
		WHERE ai_pending = TRUE AND ai_enriched = FALSE
		ORDER BY fi_score DESC
		LIMIT $1
	`, w.batchSize)
	if err != nil {
		return err
	}
	var pending []pendingItem
	for rows.Next() {
		var p pendingItem
		if err := rows.Scan(&p.ID, &p.Title, &p.Snippet, &p.Category, &p.FiScore); err != nil {
			continue
		}
		pending = append(pending, p)
	}
	rows.Close()

	if len(pending) == 0 {
		return nil
	}

	var dealItems, summaryOnlyItems []pendingItem
	for _, p := range pending {
		if isDealCategory(p.Category) {
			dealItems = append(dealItems, p)
		} else {
			summaryOnlyItems = append(summaryOnlyItems, p)
		}
	}

	enriched := int64(0)

	// ── Deal-type: full entity extraction + summary ────────────────────────
	if len(dealItems) > 0 {
		geminiQ, _, groq70bQ := w.GetQuota()
		switch {
		case w.geminiKey != "" && geminiQ < geminiDailyCap:
			enriched += w.enrichWithGemini(ctx, dealItems, true)
		case w.groqKey != "" && groq70bQ < groq70bDailyCap:
			enriched += w.enrichWithGroq(ctx, dealItems)
		default:
			w.markSkipped(ctx, dealItems)
		}
	}

	// ── Everything else that made the score cut: summary only ──────────────
	if len(summaryOnlyItems) > 0 {
		geminiQ2, _, _ := w.GetQuota()
		if w.geminiKey != "" && geminiQ2 < geminiDailyCap {
			enriched += w.enrichWithGemini(ctx, summaryOnlyItems, false)
		} else {
			w.markSkipped(ctx, summaryOnlyItems)
		}
	}

	w.StatusLastRun = time.Now()
	w.StatusItems += enriched

	if enriched > 0 {
		log.Info().Int64("enriched", enriched).Int("pending", len(pending)).Msg("enrich worker: batch done (paid pass)")
	}
	return nil
}

// enrichWithGemini sends a chunk of pending items to Gemini and persists the
// result. wantEntities controls whether companies/investors/amount are
// written back (deal-type) or only the summary (everything else).
func (w *EnrichWorker) enrichWithGemini(ctx context.Context, items []pendingItem, wantEntities bool) int64 {
	var updated int64
	for i := 0; i < len(items); i += 8 {
		geminiQ, _, _ := w.GetQuota()
		if geminiQ >= geminiDailyCap {
			w.markSkipped(ctx, items[i:])
			break
		}
		end := i + 8
		if end > len(items) {
			end = len(items)
		}
		chunk := items[i:end]

		batch := make([]batchItem, len(chunk))
		for j, p := range chunk {
			batch[j] = batchItem{ID: p.ID, Title: p.Title, Snippet: p.Snippet}
		}

		results, err := GeminiClassifyBatch(ctx, w.geminiKey, batch)
		if err != nil {
			log.Warn().Err(err).Msg("enrich worker: gemini chunk failed")
			_, _, groq70bQ := w.GetQuota()
			if w.groqKey != "" && groq70bQ < groq70bDailyCap {
				log.Info().Msg("enrich worker: falling back chunk to groq")
				w.enrichWithGroq(ctx, chunk)
			} else {
				w.markSkipped(ctx, chunk)
			}
			continue
		}
		w.addGemini(ctx, 1)

		for _, p := range chunk {
			r, ok := results[p.ID]
			if !ok || r.Summary == "" {
				_, _, groq70bQ := w.GetQuota()
				if w.groqKey != "" && groq70bQ < groq70bDailyCap {
					log.Info().Int64("id", p.ID).Msg("enrich worker: falling back single item to groq")
					w.enrichWithGroq(ctx, []pendingItem{p})
				} else {
					w.markSkipped(ctx, []pendingItem{p})
				}
				continue
			}
			if wantEntities {
				w.persistEnrichment(ctx, p.ID, r.Summary, r.KeyPoints, r.Companies, r.Investors,
					r.Amount, r.Currency, r.RoundType, r.Valuation, r.AIModel)
			} else {
				w.persistEnrichment(ctx, p.ID, r.Summary, r.KeyPoints, nil, nil, nil, "", "", nil, r.AIModel)
			}
			updated++
			if w.wsHub != nil {
				w.wsHub.Broadcast("ARTICLE_ENRICHED", map[string]any{"id": p.ID, "summary": r.Summary})
			}
			broker.GlobalSSEBroker.Broadcast("ARTICLE_ENRICHED", map[string]any{"id": p.ID, "summary": r.Summary, "title": p.Title})
		}
	}
	return updated
}

// enrichWithGroq is the fallback path for deal-type items when Gemini's
// budget is exhausted — one call per article (no batching on this model).
func (w *EnrichWorker) enrichWithGroq(ctx context.Context, items []pendingItem) int64 {
	var updated int64
	for _, p := range items {
		if _, _, g70 := w.GetQuota(); g70 >= groq70bDailyCap {
			w.markSkipped(ctx, []pendingItem{p})
			continue
		}
		result, attempts, err := GroqEnrichSingle(ctx, w.groqKey, batchItem{ID: p.ID, Title: p.Title, Snippet: p.Snippet})
		if err != nil || result == nil {
			w.markSkipped(ctx, []pendingItem{p})
			continue
		}
		w.addGroq70b(ctx, int64(attempts))
		w.persistEnrichment(ctx, p.ID, result.Summary, result.KeyPoints, result.Companies, result.Investors,
			result.Amount, result.Currency, result.RoundType, result.Valuation, result.AIModel)
		updated++
		broker.GlobalSSEBroker.Broadcast("ARTICLE_ENRICHED", map[string]any{"id": p.ID, "summary": result.Summary, "title": p.Title})
	}
	return updated
}

// persistEnrichment writes AI output back onto an already-scored, already-
// live processed_items row and clears ai_pending. Passing nil entity slices
// leaves the free keyword-derived values in place — a summary-only pass
// never wipes out data the free pass already extracted.
func (w *EnrichWorker) persistEnrichment(ctx context.Context, id int64, summary string, keyPoints, companies, investors []string,
	amount *float64, currency, roundType string, valuation *float64, aiModel string) {

	keyPointsJSON := "[]"
	if len(keyPoints) > 0 {
		if b, err := json.Marshal(keyPoints); err == nil {
			keyPointsJSON = string(b)
		}
	}

	if companies != nil || investors != nil {
		// companies/investors are NOT NULL JSONB arrays (migration 000004) —
		// only overwrite the free pass's values if the AI actually returned
		// a non-empty array; an empty array here means "AI found nothing,"
		// not "clear what the keyword pass already extracted."
		companiesJSON := toJSONArray(companies)
		investorsJSON := toJSONArray(investors)
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("enrich worker: begin tx failed")
			return
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `
			UPDATE processed_items
			SET summary = $2, key_points = $3,
			    companies = CASE WHEN $4::jsonb <> '[]'::jsonb THEN $4::jsonb ELSE companies END,
			    investors = CASE WHEN $5::jsonb <> '[]'::jsonb THEN $5::jsonb ELSE investors END,
			    amount = COALESCE($6, amount), currency = COALESCE(NULLIF($7, ''), currency),
			    round_type = COALESCE(NULLIF($8, ''), round_type), valuation = COALESCE($9, valuation),
			    ai_model_used = $10, ai_pending = FALSE, ai_enriched = TRUE
			WHERE id = $1
		`, id, summary, keyPointsJSON, companiesJSON, investorsJSON, amount, currency, roundType, valuation, aiModel)
		if err != nil {
			log.Warn().Err(err).Int64("id", id).Msg("enrich worker: persist (entities) failed")
			return
		}

		// Insert into junction table
		for _, co := range companies {
			if co != "" {
				_, _ = tx.Exec(ctx, `INSERT INTO article_entities (article_id, entity_name, entity_type) VALUES ($1, $2, 'company') ON CONFLICT DO NOTHING`, id, co)
			}
		}
		for _, inv := range investors {
			if inv != "" {
				_, _ = tx.Exec(ctx, `INSERT INTO article_entities (article_id, entity_name, entity_type) VALUES ($1, $2, 'investor') ON CONFLICT DO NOTHING`, id, inv)
			}
		}

		_ = tx.Commit(ctx)
		return
	}

	_, err := w.pool.Exec(ctx, `
		UPDATE processed_items
		SET summary = $2, key_points = $3, ai_model_used = $4, ai_pending = FALSE, ai_enriched = TRUE
		WHERE id = $1
	`, id, summary, keyPointsJSON, aiModel)
	if err != nil {
		log.Warn().Err(err).Int64("id", id).Msg("enrich worker: persist (summary-only) failed")
	}
}

// markSkipped clears ai_pending without setting ai_enriched, so the free
// pass's output stands permanently. Used when quota is exhausted or a model
// call fails — a row is never retried forever, and its free data is never
// lost while waiting.
func (w *EnrichWorker) markSkipped(ctx context.Context, items []pendingItem) {
	for _, p := range items {
		_, _ = w.pool.Exec(ctx, `UPDATE processed_items SET ai_pending = FALSE WHERE id = $1`, p.ID)
	}
}
