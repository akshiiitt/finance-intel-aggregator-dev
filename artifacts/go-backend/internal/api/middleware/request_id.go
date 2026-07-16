package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// RequestID extracts X-Request-ID from headers or generates a new one.
// It assigns the request ID to the gin context and response headers.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			bytes := make([]byte, 16)
			_, _ = rand.Read(bytes)
			reqID = hex.EncodeToString(bytes)
		}
		c.Header("X-Request-ID", reqID)
		c.Set("RequestID", reqID)
		c.Next()
	}
}
