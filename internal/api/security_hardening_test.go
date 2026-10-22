package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestSecurityHardeningIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_security_hardening_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()
	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
		"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{
		CORSOrigins:     []string{"http://localhost"},
		LoginRateLimit:  3,
		LoginRateWindow: time.Minute,
	}, db, nil, authHandler, nil, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}

	login := func() int {
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login",
			bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	for attempt := 1; attempt <= 3; attempt++ {
		if code := login(); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d", attempt, code)
		}
	}
	if code := login(); code != http.StatusTooManyRequests {
		t.Fatalf("rate limited attempt = %d", code)
	}
}

func TestSessionCookieSecureFlag(t *testing.T) {
	auth.SetCookieSecure(false)
	t.Cleanup(func() { auth.SetCookieSecure(false) })
	if auth.CookieSecure() {
		t.Fatal("secure flag should default to false")
	}
	auth.SetCookieSecure(true)
	if !auth.CookieSecure() {
		t.Fatal("secure flag not enabled")
	}
}
