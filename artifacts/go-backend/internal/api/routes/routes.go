package routes

import (
	"context"
	"net/http"
	"time"

	ginpprof "github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/financeintel/backend/internal/api/handlers"
	"github.com/financeintel/backend/internal/api/middleware"
	"github.com/financeintel/backend/internal/fxrates"
	ws "github.com/financeintel/backend/internal/websocket"
	aiworker "github.com/financeintel/backend/internal/worker/ai"
	alertworker "github.com/financeintel/backend/internal/worker/alert"
	digestworker "github.com/financeintel/backend/internal/worker/digest"
	ipoworker "github.com/financeintel/backend/internal/worker/ipo"
	marketworker "github.com/financeintel/backend/internal/worker/market"
	rssworker "github.com/financeintel/backend/internal/worker/rss"
)

// Config holds all dependencies needed to wire up routes.
type Config struct {
	Context      context.Context
	Pool         *pgxpool.Pool // API (Pooler) Pool
	DirectPool   *pgxpool.Pool // Worker (Direct) Pool
	WSHub        *ws.Hub
	RSSWorker    *rssworker.Worker
	AIWorker     *aiworker.Worker
	EnrichWorker *aiworker.EnrichWorker
	MktWorker    *marketworker.Worker
	IPOWorker    *ipoworker.Worker
	AltWorker    *alertworker.Worker
	DigestWorker *digestworker.Worker
	APIKey       string   // guards mutating routes + worker triggers, see middleware.RequireAPIKey
	CORSOrigins  []string // exact origins allowed to call this API with credentials
	Debug        bool
}

// Setup registers all routes on the provided gin engine.
func Setup(r *gin.Engine, cfg Config) {
	// ── Start live FX rate cache ───────────────────────────────────────────────
	// Polls USDINR=X from market_snapshots every 5 minutes.  This ensures all
	// handlers (analytics, scorer) use a live USD/INR rate instead of a
	// hardcoded constant.
	fxrates.Global.StartBackgroundRefresh(cfg.Context, cfg.Pool, 5*time.Minute)

	// ── Global middleware ──────────────────────────────────────────────────────
	// Recovery is registered first so it's the outermost handler — a panic in
	// any later middleware (logger, CORS, Prometheus) still yields a clean 500
	// instead of an aborted connection.
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg.CORSOrigins))
	r.Use(middleware.Prometheus())

	rl := middleware.DefaultRateLimiter(cfg.Context)
	auth := middleware.RequireAPIKey(cfg.APIKey)

	// ── pprof (debug only) ────────────────────────────────────────────────────
	if cfg.Debug {
		ginpprof.Register(r)
	}

	// ── Handlers ──────────────────────────────────────────────────────────────
	// FeedHandler gets a quota provider so GET /api/feed/stats includes live
	// AI call counts. Quota now lives on EnrichWorker — it's the only worker
	// that spends paid API calls; the free Process worker (AIWorker) never
	// touches Groq/Gemini.
	feedH := handlers.NewFeedHandler(cfg.Pool).WithQuota(cfg.EnrichWorker)
	marketH := handlers.NewMarketHandler(cfg.Pool)
	analyticsH := handlers.NewAnalyticsHandler(cfg.Pool)
	dealsH := handlers.NewDealsHandler(cfg.Pool)
	alertsH := handlers.NewAlertsHandler(cfg.Pool)
	ipoH := handlers.NewIPOHandler(cfg.Pool)
	workersH := handlers.NewWorkersHandler(
		cfg.Pool,
		cfg.RSSWorker,
		cfg.AIWorker,
		cfg.EnrichWorker,
		cfg.MktWorker,
		cfg.IPOWorker,
		cfg.AltWorker,
		cfg.DigestWorker,
	)

	// ── Health / readiness ────────────────────────────────────────────────────
	healthFn := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	r.GET("/health", healthFn)
	r.GET("/healthz", healthFn) // matches Node.js GET /healthz
	r.GET("/ready", func(c *gin.Context) {
		if err := cfg.Pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "pooler db unavailable"})
			return
		}
		if cfg.DirectPool != nil {
			if err := cfg.DirectPool.Ping(c.Request.Context()); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "direct db unavailable"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// ── Prometheus metrics ────────────────────────────────────────────────────
	// Behind the API key: the tunnel/proxy exposes the whole port, and metrics
	// leak request paths, counts, and worker internals. A scraper sends the
	// key via the X-API-Key header (or Authorization: Bearer). In development
	// (no key) it's left open for convenience.
	if cfg.APIKey != "" {
		r.GET("/metrics", auth, gin.WrapH(promhttp.Handler()))
	} else {
		r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// ── Rate-limited base group (no timeout) ──────────────────────────────────
	apiBase := r.Group("/api", rl.Middleware())

	// ── WebSocket & SSE ───────────────────────────────────────────────────────
	apiBase.GET("/ws/terminal", func(c *gin.Context) {
		cfg.WSHub.ServeWS(c.Writer, c.Request)
	})
	apiBase.GET("/feed/stream", handlers.StreamSSE)

	// ── Standard API routes (rate-limited + per-request timeout) ──────────────
	// The timeout bounds DB work so a slow query can't pin a scarce Supabase
	// pool connection until the client hangs up.
	api := apiBase.Group("", middleware.Timeout(15*time.Second))

	// ── Feed ──────────────────────────────────────────────────────────────────
	// NOTE: specific sub-routes must be registered BEFORE the /:id wildcard.
	// Entirely read-only — no auth required.
	feedG := api.Group("/feed")
	{
		feedG.GET("", feedH.GetFeed)
		feedG.GET("/stats", feedH.GetStats)
		feedG.GET("/trending", feedH.GetTrending)
		feedG.GET("/digest", feedH.GetDigest)
		feedG.GET("/search", feedH.GetSearch)
		feedG.GET("/:id", feedH.GetFeedItem)
	}

	// ── Market ────────────────────────────────────────────────────────────────
	marketG := api.Group("/market")
	{
		marketG.GET("", marketH.GetMarket)                // all latest snapshots
		marketG.GET("/history", marketH.GetSymbolHistory) // ?symbol=NIFTY&hours=24
	}

	// ── Analytics ─────────────────────────────────────────────────────────────
	analyticsG := api.Group("/analytics")
	{
		analyticsG.GET("/overview", analyticsH.GetOverview)
		analyticsG.GET("/funding", analyticsH.GetFundingLeaders)
		analyticsG.GET("/sentiment", analyticsH.GetSentimentTrend)
		analyticsG.GET("/timeline", analyticsH.GetTimeline)
	}

	// ── Deals & Entity search ─────────────────────────────────────────────────
	// Top-level routes matching Node.js routing exactly
	api.GET("/deals", dealsH.GetDeals)
	api.GET("/entity", dealsH.GetEntity)

	// ── Alerts ────────────────────────────────────────────────────────────────
	// /triggers must be registered before /:id to avoid wildcard capture.
	// Reads are open; anything that creates/edits/deletes an alert rule
	// requires the API key.
	alertsG := api.Group("/alerts")
	{
		alertsG.GET("", alertsH.ListAlerts)
		alertsG.POST("", auth, alertsH.CreateAlert)
		alertsG.GET("/triggers", alertsH.GetGlobalTriggers)        // Node.js: GET /api/alerts/triggers
		alertsG.GET("/triggers/recent", alertsH.GetRecentTriggers) // Go bonus: last 24h with alertName
		alertsG.PATCH("/:id", auth, alertsH.PatchAlert)
		alertsG.DELETE("/:id", auth, alertsH.DeleteAlert)
		alertsG.GET("/:id/triggers", alertsH.GetAlertTriggers)
	}

	// ── IPO Calendar ──────────────────────────────────────────────────────────
	// Reads are open; writes require the API key.
	ipoG := api.Group("/ipo")
	{
		ipoG.GET("", ipoH.GetIPOs)
		ipoG.POST("", auth, ipoH.CreateIPO)
		ipoG.PATCH("/:id", auth, ipoH.UpdateIPO)
		ipoG.DELETE("/:id", auth, ipoH.DeleteIPO)
	}

	// ── Workers (management + manual triggers) ────────────────────────────────
	// Status/source listing are read-only and open; every trigger spends
	// compute or paid AI budget on demand, so all of them require the API key
	// — this was previously the single biggest unauthenticated exposure in
	// the API (anyone could hit /ai/trigger repeatedly and burn quota).
	workersG := api.Group("/workers")
	{
		workersG.GET("/status", workersH.GetWorkersStatus)
		workersG.GET("/rss/sources", workersH.GetRSSSources)
		// Unified trigger — matches Node.js POST /api/workers/trigger
		workersG.POST("/trigger", auth, workersH.TriggerAll)
		// Individual triggers
		workersG.POST("/rss/trigger", auth, workersH.TriggerRSS)
		workersG.POST("/rss/trigger/:source", auth, workersH.TriggerRSSSingle)
		workersG.POST("/ai/trigger", auth, workersH.TriggerAI)
		workersG.POST("/enrich/trigger", auth, workersH.TriggerEnrich)
		workersG.POST("/market/trigger", auth, workersH.TriggerMarket)
		workersG.POST("/ipo/trigger", auth, workersH.TriggerIPO)
		workersG.POST("/alert/trigger", auth, workersH.TriggerAlert)
		workersG.POST("/digest/trigger", auth, workersH.TriggerDigest)
	}
}
