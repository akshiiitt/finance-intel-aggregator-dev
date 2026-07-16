package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a gin middleware that allows only the configured dashboard
// origin(s) to call the API with credentials.
//
// Previously this hardcoded wildcard Replit domains (*.replit.dev,
// *.repl.co, *.replit.app) alongside AllowCredentials: true — any other
// Repl app, not just this project's own dashboard, could issue credentialed
// cross-origin requests to this API. Origins now come from config
// (CORS_ALLOWED_ORIGINS), so a deploy only trusts the origin it's actually
// serving the dashboard from.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		// AllowWildcard is deliberately OFF: with AllowCredentials:true, a
		// wildcard pattern (e.g. "https://*.vercel.app") would grant
		// credentialed cross-origin access to every matching subdomain — a
		// preview deployment of any other Vercel project included. The
		// dashboard is served from one exact origin, so list that exact
		// origin in CORS_ALLOWED_ORIGINS and keep matching exact-only.
		AllowWildcard: false,
	})
}
