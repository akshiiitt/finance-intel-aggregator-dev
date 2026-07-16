package ai

import (
	"context"
	"math"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/financeintel/backend/internal/embed"
)

// NicheAnchors defines the free, embedding-based topic tags every article is
// checked against. Add a niche by adding one line here — no retraining, no
// AI call, nothing to deploy but this file.
//
// NicheThreshold is a starting point, not a calibrated constant: before
// trusting it, embed a handful of articles you know belong (and don't
// belong) to each niche and see where the cosine similarity actually splits
// for that anchor's wording. Different anchors may need different
// thresholds in practice.
var NicheAnchors = map[string]string{
	"india-fintech":     "Indian fintech funding, digital payments, neobanks, lending startups",
	"india-ev":          "electric vehicle startups India, EV manufacturing, battery technology",
	"policy-regulatory": "SEBI RBI PIB circulars, government policy, regulatory action in India",
	"global-ai":         "AI startups, foundation models, AI funding rounds globally",
	"saas-b2b":          "B2B SaaS startups, enterprise software funding",
	"ecommerce-retail":  "ecommerce, D2C brands, retail startups in India",
}

const NicheThreshold = 0.45 // cosine similarity — see calibration note above

// nicheVectors caches the embedded anchors so they're computed once per
// process, not once per article. It's guarded by nicheMu because the AI
// worker starts with WithStartImmediately(), so the first free-pass cycle
// (which reads it via TagNiches) can run concurrently with InitNicheAnchors
// writing it — an unsynchronized read/write the race detector flags.
var (
	nicheMu      sync.RWMutex
	nicheVectors map[string][]float32
)

// InitNicheAnchors embeds every anchor phrase once at worker startup. If the
// sidecar is unavailable, niche tagging is simply skipped this run — it's
// additive, never load-bearing, and never blocks the free feed from working.
func InitNicheAnchors(ctx context.Context, ec *embed.Client) {
	names := make([]string, 0, len(NicheAnchors))
	phrases := make([]string, 0, len(NicheAnchors))
	for name, phrase := range NicheAnchors {
		names = append(names, name)
		phrases = append(phrases, phrase)
	}

	vectors, err := ec.Embed(ctx, phrases)
	if err != nil {
		log.Warn().Err(err).Msg("niches: failed to embed anchors, niche tagging disabled this run")
		return
	}

	vecMap := make(map[string][]float32, len(names))
	for i, name := range names {
		vecMap[name] = vectors[i]
	}
	nicheMu.Lock()
	nicheVectors = vecMap
	nicheMu.Unlock()
	log.Info().Int("niches", len(vecMap)).Msg("niches: anchors embedded")
}

// TagNiches returns every niche whose anchor is within NicheThreshold cosine
// similarity of the given article embedding. Reuses the vector already
// computed for dedup — this never triggers an extra embedding call.
func TagNiches(v []float32) []string {
	nicheMu.RLock()
	defer nicheMu.RUnlock()
	if v == nil || nicheVectors == nil {
		return nil
	}
	var out []string
	for name, anchor := range nicheVectors {
		if cosineSimilarity(v, anchor) >= NicheThreshold {
			out = append(out, name)
		}
	}
	return out
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
