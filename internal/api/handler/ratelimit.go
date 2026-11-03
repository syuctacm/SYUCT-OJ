package handler

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type rateBucket struct {
	count   int
	resetAt time.Time
}

type requestRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
}

var defaultRequestRateLimiter = &requestRateLimiter{buckets: map[string]*rateBucket{}}

func (l *requestRateLimiter) allow(key string, limit int, window time.Duration) (bool, int) {
	if limit <= 0 {
		return true, 0
	}
	if window <= 0 {
		window = time.Minute
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket := l.buckets[key]
	if bucket == nil || now.After(bucket.resetAt) {
		bucket = &rateBucket{resetAt: now.Add(window)}
		l.buckets[key] = bucket
	}
	bucket.count++
	exceeded := bucket.count > limit
	retryAfter := int(bucket.resetAt.Sub(now).Seconds()) + 1
	if len(l.buckets) > 10000 {
		for candidate, entry := range l.buckets {
			if now.After(entry.resetAt) {
				delete(l.buckets, candidate)
			}
		}
	}
	return !exceeded, retryAfter
}

func configuredBool(c *gin.Context, key string, fallback bool) bool {
	rawDB, ok := c.Get(RequestDatabaseKey)
	if !ok {
		return fallback
	}
	db, ok := rawDB.(*mongo.Database)
	if !ok || db == nil {
		return fallback
	}
	var doc struct {
		Value any `bson:"value"`
	}
	if err := db.Collection("system").FindOne(c.Request.Context(), bson.M{"_id": key}).Decode(&doc); err != nil {
		return fallback
	}
	value, ok := anyBool(doc.Value)
	if !ok {
		return fallback
	}
	return value
}

func rateLimitKey(c *gin.Context, uid int64) string {
	if configuredBool(c, settingLimitByUser, false) {
		return "user:" + strconv.FormatInt(uid, 10)
	}
	return "ip:" + c.ClientIP()
}

func enforceRateLimit(c *gin.Context, uid int64, bucket, ipSettingKey, userSettingKey string, ipFallback, userFallback int) bool {
	settingKey := ipSettingKey
	fallback := ipFallback
	if configuredBool(c, settingLimitByUser, false) {
		settingKey = userSettingKey
		fallback = userFallback
	}
	limit := configuredLimitInt(c, settingKey, fallback)
	ok, retryAfter := defaultRequestRateLimiter.allow(bucket+"|"+rateLimitKey(c, uid), limit, time.Minute)
	if ok {
		return true
	}
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, please try again later"})
	return false
}
