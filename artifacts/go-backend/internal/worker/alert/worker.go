package alert

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Worker evaluates active alert rules against recently processed articles.
type Worker struct {
	pool          *pgxpool.Pool
	StatusLastRun time.Time
	StatusMatches int64
}

// New creates an alert worker.
func New(pool *pgxpool.Pool) *Worker {
	return &Worker{pool: pool}
}

// Run evaluates all active alerts against articles processed in the last 2 minutes.
// Called by the scheduler every ALERT_INTERVAL_MINUTES.
func (w *Worker) Run(ctx context.Context) error {
	// Fetch active alerts
	alertRows, err := w.pool.Query(ctx, `
		SELECT id, name, type, conditions
		FROM alerts
		WHERE is_active = TRUE
		LIMIT 100
	`)
	if err != nil {
		return err
	}
	defer alertRows.Close()

	type alertRule struct {
		ID         int64
		Name       string
		Type       string
		Conditions map[string]interface{}
	}

	var rules []alertRule
	for alertRows.Next() {
		var a alertRule
		var condJSON string
		if err := alertRows.Scan(&a.ID, &a.Name, &a.Type, &condJSON); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(condJSON), &a.Conditions)
		rules = append(rules, a)
	}
	alertRows.Close()

	if len(rules) == 0 {
		return nil
	}

	// Fetch articles processed in the last 2 minutes
	// Use numeric types directly — avoids the intermediate string conversion
	twoMinAgo := time.Now().Add(-2 * time.Minute)
	articleRows, err := w.pool.Query(ctx, `
		SELECT id, title,
		       COALESCE(summary, '')          AS summary,
		       COALESCE(jsonb_to_csv(companies), '') AS companies,
		       COALESCE(jsonb_to_csv(investors), '') AS investors,
		       COALESCE(category, '')         AS category,
		       COALESCE(source, '')           AS source,
		       COALESCE(fi_score::numeric, 0)::float8 AS fi_score,
		       COALESCE(amount, 0)::float8            AS amount
		FROM processed_items
		WHERE fetched_at >= $1
		ORDER BY fetched_at DESC
		LIMIT 50
	`, twoMinAgo)
	if err != nil {
		return err
	}
	defer articleRows.Close()

	type article struct {
		ID        int64
		Title     string
		Summary   string
		Companies string
		Investors string
		Category  string
		Source    string
		FiScore   float64
		Amount    float64
	}

	var articles []article
	for articleRows.Next() {
		var a article
		if err := articleRows.Scan(
			&a.ID, &a.Title, &a.Summary, &a.Companies, &a.Investors,
			&a.Category, &a.Source, &a.FiScore, &a.Amount,
		); err != nil {
			continue
		}
		articles = append(articles, a)
	}
	articleRows.Close()

	if len(articles) == 0 {
		return nil
	}

	matches := 0
	for _, rule := range rules {
		for _, art := range articles {
			if !matchesAlert(rule.Conditions, art.Title, art.Summary,
				art.Companies, art.Investors, art.Category, art.FiScore, art.Amount) {
				continue
			}

			// Insert trigger — unique constraint (alert_id, article_id) prevents duplicates
			_, err := w.pool.Exec(ctx, `
				INSERT INTO alert_triggers (alert_id, article_id, title, source, category, fi_score)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (alert_id, article_id) DO NOTHING
			`, rule.ID, art.ID, art.Title, art.Source, art.Category, art.FiScore)
			if err != nil {
				log.Warn().Err(err).Int64("alert", rule.ID).Msg("alert worker: insert trigger failed")
				continue
			}

			// Update alert metadata (best-effort — errors non-fatal)
			_, _ = w.pool.Exec(ctx, `
				UPDATE alerts
				SET last_triggered = NOW(),
				    trigger_count  = trigger_count + 1
				WHERE id = $1
			`, rule.ID)

			matches++
		}
	}

	w.StatusLastRun = time.Now()
	if matches > 0 {
		log.Info().Int("matches", matches).Msg("alert worker: triggers fired")
	}
	return nil
}

// matchesAlert returns true when an article satisfies all the alert's conditions.
// Matching logic mirrors the Node.js alert worker exactly.
func matchesAlert(
	conditions map[string]interface{},
	title, summary, companies, investors, category string,
	fiScore, amount float64,
) bool {
	text := strings.ToLower(title + " " + summary + " " + companies + " " + investors)
	matched := false

	// Keyword match (OR across keywords)
	if kws, ok := toStrSlice(conditions["keywords"]); ok && len(kws) > 0 {
		for _, kw := range kws {
			if strings.Contains(text, strings.ToLower(kw)) {
				matched = true
				break
			}
		}
	}

	// Company match (OR across companies)
	if !matched {
		if cos, ok := toStrSlice(conditions["companies"]); ok && len(cos) > 0 {
			for _, co := range cos {
				if strings.Contains(strings.ToLower(companies), strings.ToLower(co)) {
					matched = true
					break
				}
			}
		}
	}

	// Category match (OR across categories)
	if !matched {
		if cats, ok := toStrSlice(conditions["categories"]); ok && len(cats) > 0 {
			for _, cat := range cats {
				if strings.ToLower(category) == strings.ToLower(cat) {
					matched = true
					break
				}
			}
		}
	}

	if !matched {
		return false
	}

	// Secondary numeric filters — applied after primary match
	if minFi, ok := toFloat(conditions["minFiScore"]); ok && fiScore < minFi {
		return false
	}
	if minCr, ok := toFloat(conditions["minAmountCr"]); ok {
		// amount is raw INR value; convert to crore for comparison
		amountCr := amount / 10_000_000.0
		if amountCr < minCr {
			return false
		}
	}

	return true
}

// toStrSlice safely coerces a JSON array value to []string.
func toStrSlice(v interface{}) ([]string, bool) {
	if v == nil {
		return nil, false
	}
	raw, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out, true
}

// toFloat safely coerces a JSON number value to float64.
// Handles both float64 and integer types produced by json.Unmarshal.
func toFloat(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case string:
		// Fallback: parse string representation (e.g. "75.0")
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
