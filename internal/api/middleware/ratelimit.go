package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	count   int
	resetAt time.Time
}

// RateLimit applies a fixed-window per-IP limit to the given request paths. The
// limiter is in-memory and per-process, which is enough for a single API
// instance; a shared store would be needed behind multiple replicas.
func RateLimit(limit int, window time.Duration, paths ...string) gin.HandlerFunc {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	targets := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		targets[path] = struct{}{}
	}
	var mu sync.Mutex
	buckets := map[string]*rateBucket{}
	return func(c *gin.Context) {
		if _, ok := targets[c.Request.URL.Path]; !ok {
			c.Next()
			return
		}
		now := time.Now()
		key := c.Request.URL.Path + "|" + c.ClientIP()
		mu.Lock()
		bucket := buckets[key]
		if bucket == nil || now.After(bucket.resetAt) {
			bucket = &rateBucket{resetAt: now.Add(window)}
			buckets[key] = bucket
		}
		bucket.count++
		exceeded := bucket.count > limit
		retryAfter := int(bucket.resetAt.Sub(now).Seconds()) + 1
		if len(buckets) > 10000 {
			for candidate, entry := range buckets {
				if now.After(entry.resetAt) {
					delete(buckets, candidate)
				}
			}
		}
		mu.Unlock()
		if exceeded {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, please try again later"})
			return
		}
		c.Next()
	}
}
