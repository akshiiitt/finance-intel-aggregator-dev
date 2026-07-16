package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	aiworker "github.com/financeintel/backend/internal/worker/ai"
	alertworker "github.com/financeintel/backend/internal/worker/alert"
	digestworker "github.com/financeintel/backend/internal/worker/digest"
	ipoworker "github.com/financeintel/backend/internal/worker/ipo"
	marketworker "github.com/financeintel/backend/internal/worker/market"
	rssworker "github.com/financeintel/backend/internal/worker/rss"
)

// WorkersHandler handles all /api/workers/* routes (status + manual triggers).
type WorkersHandler struct {
	pool         *pgxpool.Pool
	rssW         *rssworker.Worker
	aiW          *aiworker.Worker
	enrichW      *aiworker.EnrichWorker
	marketW      *marketworker.Worker
	ipoW         *ipoworker.Worker
	alertW       *alertworker.Worker
	digestW      *digestworker.Worker
	isTriggering int32 // concurrency guard for manual trigger actions
}

// NewWorkersHandler creates a workers handler.
func NewWorkersHandler(
	pool *pgxpool.Pool,
	rss *rssworker.Worker,
	ai *aiworker.Worker,
	enrich *aiworker.EnrichWorker,
	mkt *marketworker.Worker,
	ipo *ipoworker.Worker,
	alt *alertworker.Worker,
	dig *digestworker.Worker,
) *WorkersHandler {
	return &WorkersHandler{
		pool:    pool,
		rssW:    rss,
		aiW:     ai,
		enrichW: enrich,
		marketW: mkt,
		ipoW:    ipo,
		alertW:  alt,
		digestW: dig,
	}
}

// isoOrNull formats a time as RFC3339 string, or nil if zero.
func isoOrNull(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// workerStatus infers a status string from the last run time.
func workerStatus(lastRun time.Time) string {
	if lastRun.IsZero() {
		return "idle"
	}
	return "idle" // workers are event-driven; "running" is set during execution
}

// GetWorkersStatus handles GET /api/workers/status
//
//	{
//	  workers: [{name, status, lastRun, itemsProcessed}],
//	  lastRun, nextRun,
//	  quota: {geminiUsed, geminiLimit, groq8bUsed, groq8bLimit, groq70bUsed, groq70bLimit},
//	  aiBreakdown: { "model-name": count }
//	}
//
// "AI Worker" here is the free classify/dedup/score pass; "Enrich Worker" is
// the separate paid pass that only touches ai_pending rows — see
// internal/worker/ai/worker.go and enrich.go for why they're split.
func (h *WorkersHandler) GetWorkersStatus(c *gin.Context) {
	ctx := c.Request.Context()

	geminiCalls, groq8bCalls, groq70bCalls := h.enrichW.GetQuota()

	type workerOut struct {
		Name           string  `json:"name"`
		Status         string  `json:"status"`
		LastRun        *string `json:"lastRun"`
		ItemsProcessed int64   `json:"itemsProcessed"`
	}

	workers := []workerOut{
		{
			Name:           "RSS Worker",
			Status:         workerStatus(h.rssW.StatusLastRun),
			LastRun:        isoOrNull(h.rssW.StatusLastRun),
			ItemsProcessed: atomic.LoadInt64(&h.rssW.StatusItems),
		},
		{
			Name:           "AI Worker",
			Status:         workerStatus(h.aiW.StatusLastRun),
			LastRun:        isoOrNull(h.aiW.StatusLastRun),
			ItemsProcessed: atomic.LoadInt64(&h.aiW.StatusItems),
		},
		{
			Name:           "Enrich Worker",
			Status:         workerStatus(h.enrichW.StatusLastRun),
			LastRun:        isoOrNull(h.enrichW.StatusLastRun),
			ItemsProcessed: h.enrichW.StatusItems,
		},
		{
			Name:           "Market Worker",
			Status:         workerStatus(h.marketW.StatusLastRun),
			LastRun:        isoOrNull(h.marketW.StatusLastRun),
			ItemsProcessed: atomic.LoadInt64(&h.marketW.StatusUpdated),
		},
		{
			Name:           "IPO Worker",
			Status:         workerStatus(h.ipoW.StatusLastRun),
			LastRun:        isoOrNull(h.ipoW.StatusLastRun),
			ItemsProcessed: 0,
		},
		{
			Name:           "Alert Worker",
			Status:         workerStatus(h.alertW.StatusLastRun),
			LastRun:        isoOrNull(h.alertW.StatusLastRun),
			ItemsProcessed: h.alertW.StatusMatches,
		},
		{
			Name:           "Digest Worker",
			Status:         workerStatus(h.digestW.StatusLastRun),
			LastRun:        isoOrNull(h.digestW.StatusLastRun),
			ItemsProcessed: 0,
		},
	}

	// lastRun = most recent worker run across all workers (proxy for scheduler lastRun)
	var latestRun time.Time
	for _, w := range []time.Time{
		h.rssW.StatusLastRun, h.aiW.StatusLastRun, h.enrichW.StatusLastRun,
		h.marketW.StatusLastRun, h.ipoW.StatusLastRun, h.alertW.StatusLastRun, h.digestW.StatusLastRun,
	} {
		if w.After(latestRun) {
			latestRun = w
		}
	}

	// AI model breakdown (today)
	today := time.Now().Truncate(24 * time.Hour)
	aiRows, _ := h.pool.Query(ctx, `
		SELECT ai_model_used, COUNT(*) as cnt
		FROM processed_items
		WHERE fetched_at >= $1 AND ai_model_used IS NOT NULL
		GROUP BY ai_model_used
	`, today)
	aiBreakdown := map[string]int64{}
	if aiRows != nil {
		defer aiRows.Close()
		for aiRows.Next() {
			var model string
			var cnt int64
			if err := aiRows.Scan(&model, &cnt); err == nil {
				aiBreakdown[model] = cnt
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"workers": workers,
		"lastRun": isoOrNull(latestRun),
		"nextRun": nil, // Go scheduler doesn't expose next-run time
		"quota": gin.H{
			"geminiUsed":   geminiCalls,
			"geminiLimit":  1500,
			"groq8bUsed":   groq8bCalls,
			"groq8bLimit":  14400,
			"groq70bUsed":  groq70bCalls,
			"groq70bLimit": 1000,
		},
		"aiBreakdown": aiBreakdown,
	})
}

// TriggerAll handles POST /api/workers/trigger
//
//	Request body: { source?: string }
//	Response:     { success: bool, message: string, itemsFound: int|null }
//
// If source is provided, only that RSS source is fetched. Otherwise all
// feeds + market data are fetched, the free AI pass processes the batch,
// then the paid Enrich pass runs once immediately after (instead of waiting
// for its own schedule) so a manual trigger feels complete end-to-end.
func (h *WorkersHandler) TriggerAll(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{
			"success":    false,
			"message":    "Another manual worker execution is already in progress",
			"itemsFound": nil,
		})
		return
	}
	defer atomic.StoreInt32(&h.isTriggering, 0)

	var body struct {
		Source string `json:"source"`
	}
	_ = c.ShouldBindJSON(&body)

	ctx := c.Request.Context()

	var itemsFound int

	if body.Source != "" {
		n, err := h.rssW.RunSingle(ctx, body.Source)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success":    false,
				"message":    err.Error(),
				"itemsFound": nil,
			})
			return
		}
		itemsFound = n
	} else {
		before := atomic.LoadInt64(&h.rssW.StatusItems)
		if err := h.rssW.Run(ctx); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success":    false,
				"message":    err.Error(),
				"itemsFound": nil,
			})
			return
		}
		after := atomic.LoadInt64(&h.rssW.StatusItems)
		itemsFound = int(after - before)

		// Market data runs concurrently with AI (no dependency)
		go func() {
			mCtx, mCancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer mCancel()
			_ = h.marketW.Run(mCtx)
		}()
	}

	aiBefore := atomic.LoadInt64(&h.aiW.StatusItems)
	_ = h.aiW.Run(ctx)
	aiAfter := atomic.LoadInt64(&h.aiW.StatusItems)
	processed := aiAfter - aiBefore

	// Immediately spend whatever the free pass just flagged, so a manual
	// trigger produces enriched entities/summaries without waiting for the
	// Enrich worker's own schedule.
	_ = h.enrichW.Run(ctx)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    fmt.Sprintf("Fetched %d new items, processed %d", itemsFound, processed),
		"itemsFound": itemsFound,
	})
}

// TriggerRSS handles POST /api/workers/rss/trigger
func (h *WorkersHandler) TriggerRSS(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = h.rssW.Run(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "RSS worker triggered", "startedAt": time.Now().UTC().Format(time.RFC3339)})
}

// TriggerAI handles POST /api/workers/ai/trigger — runs the free
// classify/dedup/score pass only. Use /api/workers/enrich/trigger to spend
// paid API calls on whatever it flags.
func (h *WorkersHandler) TriggerAI(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = h.aiW.Run(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "AI worker (free pass) triggered", "startedAt": time.Now().UTC().Format(time.RFC3339)})
}

// TriggerEnrich handles POST /api/workers/enrich/trigger — spends paid API
// calls on whatever the free pass has flagged ai_pending = TRUE.
func (h *WorkersHandler) TriggerEnrich(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = h.enrichW.Run(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "Enrich worker (paid pass) triggered", "startedAt": time.Now().UTC().Format(time.RFC3339)})
}

// TriggerMarket handles POST /api/workers/market/trigger
func (h *WorkersHandler) TriggerMarket(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = h.marketW.Run(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "Market worker triggered", "startedAt": time.Now().UTC().Format(time.RFC3339)})
}

// TriggerIPO handles POST /api/workers/ipo/trigger
func (h *WorkersHandler) TriggerIPO(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = h.ipoW.Run(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "IPO worker triggered", "startedAt": time.Now().UTC().Format(time.RFC3339)})
}

// TriggerAlert handles POST /api/workers/alert/trigger
func (h *WorkersHandler) TriggerAlert(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = h.alertW.Run(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "Alert worker triggered", "startedAt": time.Now().UTC().Format(time.RFC3339)})
}

// TriggerDigest handles POST /api/workers/digest/trigger — generates
// today's briefing immediately instead of waiting for DIGEST_HOUR_UTC.
func (h *WorkersHandler) TriggerDigest(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = h.digestW.Run(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "Digest worker triggered", "startedAt": time.Now().UTC().Format(time.RFC3339)})
}

// TriggerRSSSingle handles POST /api/workers/rss/trigger/:source
func (h *WorkersHandler) TriggerRSSSingle(c *gin.Context) {
	if !atomic.CompareAndSwapInt32(&h.isTriggering, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"message": "Another manual worker execution is already in progress"})
		return
	}
	source := c.Param("source")
	go func() {
		defer atomic.StoreInt32(&h.isTriggering, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = h.rssW.RunSingle(ctx, source)
	}()
	c.JSON(http.StatusAccepted, gin.H{
		"message": "RSS single source triggered",
		"source":  source,
	})
}

// GetRSSSources handles GET /api/workers/rss/sources
// Returns all configured RSS source names and metadata.
func (h *WorkersHandler) GetRSSSources(c *gin.Context) {
	type sourceOut struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Region   string `json:"region"`
		Category string `json:"category"`
		Trust    int    `json:"trust"`
	}
	out := make([]sourceOut, len(rssworker.FeedSources))
	for i, s := range rssworker.FeedSources {
		out[i] = sourceOut{
			Name:     s.Name,
			URL:      s.URL,
			Region:   s.Region,
			Category: s.Category,
			Trust:    s.Trust,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"sources": out,
		"total":   len(out),
	})
}
