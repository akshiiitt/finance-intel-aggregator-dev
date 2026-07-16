// Package digest generates the real daily morning briefing — the one thing
// the old /api/feed/digest endpoint never actually did (it just returned a
// templated view of the day's feed). All the expensive work (dedup, scoring,
// niche tagging) already happened for free upstream in the AI Process
// worker; this is the single AI call per day that turns the result into
// something readable in 30 seconds.
package digest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Worker builds one row in daily_digests per day, if one doesn't already
// exist. Safe to call more than once a day — it's a no-op after the first
// successful run that day.
type Worker struct {
	pool          *pgxpool.Pool
	geminiKey     string
	StatusLastRun time.Time
}

// New creates the digest worker.
func New(pool *pgxpool.Pool, geminiKey string) *Worker {
	return &Worker{pool: pool, geminiKey: geminiKey}
}

type topStory struct {
	ID       int64
	Title    string
	Summary  string
	Category string
	Source   string
	FiScore  float64
}

// Run builds today's digest if one doesn't already exist.
func (w *Worker) Run(ctx context.Context) error {
	today := time.Now().UTC().Format("2006-01-02")

	var exists bool
	_ = w.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM daily_digests WHERE digest_date = $1)`, today).Scan(&exists)
	if exists {
		return nil
	}

	rows, err := w.pool.Query(ctx, `
		SELECT id, title, COALESCE(summary, '') as summary, COALESCE(category, '') as category,
		       source, COALESCE(fi_score, 0)::float8 as fi_score
		FROM processed_items
		WHERE fetched_at > now() - interval '24 hours'
		ORDER BY fi_score DESC
		LIMIT 15
	`)
	if err != nil {
		return err
	}
	var stories []topStory
	for rows.Next() {
		var s topStory
		if err := rows.Scan(&s.ID, &s.Title, &s.Summary, &s.Category, &s.Source, &s.FiScore); err != nil {
			continue
		}
		stories = append(stories, s)
	}
	rows.Close()

	if len(stories) == 0 {
		w.StatusLastRun = time.Now()
		return nil
	}

	content := w.generate(ctx, stories)

	ids := make([]int64, len(stories))
	for i, s := range stories {
		ids[i] = s.ID
	}

	_, err = w.pool.Exec(ctx, `
		INSERT INTO daily_digests (digest_date, content, top_story_ids)
		VALUES ($1, $2, $3)
		ON CONFLICT (digest_date) DO NOTHING
	`, today, content, ids)
	if err != nil {
		log.Error().Err(err).Msg("digest worker: failed to save digest")
		return err
	}

	w.StatusLastRun = time.Now()
	log.Info().Int("stories", len(stories)).Msg("digest worker: generated today's briefing")
	return nil
}

// generate calls Gemini once with the day's top stories. Falls back to a
// plain templated digest (no AI call, still useful) if no key is
// configured or the call fails — the digest must never be blank just
// because the AI provider is down.
func (w *Worker) generate(ctx context.Context, stories []topStory) string {
	if w.geminiKey == "" {
		return w.fallbackDigest(stories)
	}

	var sb strings.Builder
	for i, s := range stories {
		fmt.Fprintf(&sb, "%d. [%s/%s] %s — %s\n", i+1, s.Source, s.Category, s.Title, s.Summary)
	}

	prompt := fmt.Sprintf(`You are writing a morning financial briefing for an Indian investor/entrepreneur.
Based on today's top stories (sorted by importance):

%s

Write a crisp morning briefing covering:
1. India market/business summary (2-3 sentences)
2. Top funding rounds or deals, if any
3. Key policy/regulatory development, if any
4. One notable global signal relevant to India
5. One-line sentiment call: Bullish / Neutral / Bearish, and why

Keep it under 250 words. No fluff. Write like you're briefing a smart founder.`, sb.String())

	text, err := callGeminiText(ctx, w.geminiKey, prompt)
	if err != nil || text == "" {
		log.Warn().Err(err).Msg("digest worker: gemini generation failed, using fallback digest")
		return w.fallbackDigest(stories)
	}
	return text
}

// fallbackDigest is a zero-cost digest built from the same stories — used
// when no Gemini key is configured or the call fails. Never AI-written, but
// never blank either.
func (w *Worker) fallbackDigest(stories []topStory) string {
	var sb strings.Builder
	sb.WriteString("Today's top stories:\n\n")
	for i, s := range stories {
		if i >= 8 {
			break
		}
		fmt.Fprintf(&sb, "- [%s] %s (%s)\n", strings.ToUpper(s.Category), s.Title, s.Source)
	}
	return sb.String()
}
