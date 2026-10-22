package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	}
	for header, value := range want {
		if got := recorder.Header().Get(header); got != value {
			t.Fatalf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestBodyLimitRejectsOversizedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(16))
	router.POST("/", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	small := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, small)
	if recorder.Code != http.StatusOK {
		t.Fatalf("small body = %d", recorder.Code)
	}

	large := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bytes.Repeat([]byte("x"), 64)))
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, large)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body = %d", recorder.Code)
	}
}

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(2, time.Minute, "/login"))
	router.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/other", func(c *gin.Context) { c.Status(http.StatusOK) })

	call := func(ip string) int {
		request := httptest.NewRequest(http.MethodPost, "/login", nil)
		request.Header.Set("X-Forwarded-For", ip)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusTooManyRequests && recorder.Header().Get("Retry-After") == "" {
			t.Fatalf("429 without Retry-After")
		}
		return recorder.Code
	}
	if code := call("10.0.0.1"); code != http.StatusOK {
		t.Fatalf("first = %d", code)
	}
	if code := call("10.0.0.1"); code != http.StatusOK {
		t.Fatalf("second = %d", code)
	}
	if code := call("10.0.0.1"); code != http.StatusTooManyRequests {
		t.Fatalf("third = %d", code)
	}
	if code := call("10.0.0.2"); code != http.StatusOK {
		t.Fatalf("other ip = %d", code)
	}
	other := httptest.NewRequest(http.MethodGet, "/other", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, other)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unlimited path = %d", recorder.Code)
	}
}

func TestRateLimitRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(1, 30*time.Second, "/login"))
	router.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	value := recorder.Header().Get("Retry-After")
	if value == "" || value == "0" {
		t.Fatalf("Retry-After = %q", value)
	}
	if strings.TrimSpace(fmt.Sprint(recorder.Code)) != "429" {
		t.Fatalf("status = %d", recorder.Code)
	}
}
