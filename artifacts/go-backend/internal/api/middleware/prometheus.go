package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "financeintel_http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "financeintel_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	wsConnectedClients = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "financeintel_ws_connected_clients",
		Help: "Number of currently connected WebSocket clients.",
	})

	workerRunsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "financeintel_worker_runs_total",
			Help: "Total number of worker run invocations.",
		},
		[]string{"worker", "status"},
	)

	workerItemsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "financeintel_worker_items_processed_total",
			Help: "Total items processed by each worker.",
		},
		[]string{"worker"},
	)
)

// Prometheus returns a gin middleware that records request metrics.
// Mount the /metrics endpoint separately using promhttp.Handler().
func Prometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip the metrics endpoint itself to avoid recursion.
		if c.FullPath() == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}

		status := strconv.Itoa(c.Writer.Status())
		elapsed := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(elapsed)
	}
}

// IncWSClients increments the WebSocket connected-client gauge.
func IncWSClients() { wsConnectedClients.Inc() }

// DecWSClients decrements the WebSocket connected-client gauge.
func DecWSClients() { wsConnectedClients.Dec() }

// IncWorkerRun records one worker invocation. status is "ok" or "error".
func IncWorkerRun(worker, status string) {
	workerRunsTotal.WithLabelValues(worker, status).Inc()
}

// AddWorkerItems records how many items a worker processed in one run.
func AddWorkerItems(worker string, n float64) {
	workerItemsProcessed.WithLabelValues(worker).Add(n)
}
