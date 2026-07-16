package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAPIKey returns a gin middleware that rejects any request whose
// X-API-Key header (or "Bearer <key>" Authorization header) doesn't match
// apiKey. Use it on mutating routes (POST/PATCH/DELETE) and worker triggers
// — this is a single-tenant personal tool, so one shared secret is the
// right amount of auth, not full user accounts.
//
// If apiKey is empty, every request is rejected — there is deliberately no
// "auth disabled" mode. Config.Load() already refuses to start the server
// in production without an API_KEY set (see internal/config), so reaching
// this state in production means misconfiguration, not an open API.
func RequireAPIKey(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "server misconfigured: API_KEY is not set",
			})
			return
		}

		provided := c.GetHeader("X-API-Key")
		if provided == "" {
			if auth := c.GetHeader("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
				provided = auth[7:]
			}
		}

		// Constant-time compare — this key protects delete/trigger routes,
		// so a timing side-channel on the comparison is worth closing even
		// though the threat model here is modest.
		if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing or invalid API key",
			})
			return
		}

		c.Next()
	}
}
