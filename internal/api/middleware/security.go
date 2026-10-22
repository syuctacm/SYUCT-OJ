package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds the baseline hardening headers for API responses. A CSP
// is intentionally left to the reverse proxy: the SPA renders upstream
// templates with inline styles/attributes.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

// DefaultBodyLimit is the JSON/API request cap; multipart uploads set their own
// larger limits in the handlers.
const DefaultBodyLimit int64 = 8 << 20

// BodyLimit caps non-multipart request bodies so a malformed or hostile client
// cannot buffer unbounded JSON in memory.
func BodyLimit(limit int64) gin.HandlerFunc {
	if limit <= 0 {
		limit = DefaultBodyLimit
	}
	return func(c *gin.Context) {
		if c.Request.Body != nil && !strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
