package handler

import (
	"testing"
	"time"
)

func TestRequestRateLimiterWindow(t *testing.T) {
	limiter := &requestRateLimiter{buckets: map[string]*rateBucket{}}
	window := 20 * time.Millisecond
	if ok, _ := limiter.allow("submission|user:7", 2, window); !ok {
		t.Fatal("first request should pass")
	}
	if ok, _ := limiter.allow("submission|user:7", 2, window); !ok {
		t.Fatal("second request should pass")
	}
	if ok, retryAfter := limiter.allow("submission|user:7", 2, window); ok || retryAfter < 1 {
		t.Fatalf("third request = ok %v retryAfter %d", ok, retryAfter)
	}
	time.Sleep(window + 5*time.Millisecond)
	if ok, _ := limiter.allow("submission|user:7", 2, window); !ok {
		t.Fatal("request after window should pass")
	}
}
