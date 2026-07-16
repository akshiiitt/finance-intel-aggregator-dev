package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout bounds how long any single request's downstream work (DB queries,
// upstream calls) may run by deriving a deadline on the request context.
//
// It does NOT itself write a response on expiry — that avoids the data race
// gin's stock timeout middleware has when a slow handler writes concurrently
// with the timeout writer. Instead, handlers pass c.Request.Context() to their
// queries, so on expiry those queries return context.DeadlineExceeded and the
// handler returns its normal error path. The point is to stop a slow/blocked
// query from pinning a scarce Supabase pool connection until the client hangs
// up — critical on the free-tier connection budget.
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
