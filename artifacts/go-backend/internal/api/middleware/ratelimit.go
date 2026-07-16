package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ipBucket tracks the per-IP request count in a sliding window.
type ipBucket struct {
	count    int
	windowAt time.Time
}

// RateLimiter is an in-memory per-IP token bucket limiter.
// It allows `max` requests per `window` duration per IP address.
// This is a simple implementation suitable for a single-server deployment.
// For multi-replica, replace with a Redis-backed limiter.
type RateLimiter struct {
	mu     sync.Mutex
	ips    map[string]*ipBucket
	max    int
	window time.Duration
}

// NewRateLimiter creates a rate limiter with the given limits.
func NewRateLimiter(ctx context.Context, max int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		ips:    make(map[string]*ipBucket, 256),
		max:    max,
		window: window,
	}
	// Background cleanup — evict stale buckets every minute.
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.evict()
			case <-ctx.Done():
				return
			}
		}
	}()
	return rl
}

func (rl *RateLimiter) evict() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	for ip, b := range rl.ips {
		if b.windowAt.Before(cutoff) {
			delete(rl.ips, ip)
		}
	}
}

// Allow returns true if the request from ip is within the rate limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.ips[ip]
	if !ok {
		rl.ips[ip] = &ipBucket{count: 1, windowAt: time.Now()}
		return true
	}

	if time.Since(b.windowAt) > rl.window {
		b.count = 1
		b.windowAt = time.Now()
		return true
	}

	if b.count >= rl.max {
		return false
	}
	b.count++
	return true
}

// Middleware returns a gin handler that enforces the rate limit.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"retryIn": rl.window.String(),
			})
			return
		}
		c.Next()
	}
}

// DefaultRateLimiter creates a limiter: 120 requests per minute per IP.
func DefaultRateLimiter(ctx context.Context) *RateLimiter {
	return NewRateLimiter(ctx, 120, time.Minute)
}
