package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestPublicSettingsIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_settings_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com", "mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, nil, userRepo, adminHandler, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	type settingsPayload struct {
		Data struct {
			Bulletin string `json:"bulletin"`
		} `json:"data"`
	}
	get := func(path string, cookies []*http.Cookie) (int, settingsPayload) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var payload settingsPayload
		if resp.StatusCode == http.StatusOK {
			if err := decodeJSON(resp.Body, &payload); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, payload
	}
	put := func(body string, cookies []*http.Cookie) int {
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	status, payload := get("/api/v1/settings", nil)
	if status != http.StatusOK || payload.Data.Bulletin != "" {
		t.Fatalf("initial public settings = %d %+v", status, payload.Data)
	}

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")

	if status := put(`{"bulletin":"# 公告"}`, aliceCookies); status != http.StatusForbidden {
		t.Fatalf("non-admin update = %d", status)
	}
	if status := put(`{"bulletin":"# 公告"}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous update = %d", status)
	}
	if status := put(`{"bulletin":"`+strings.Repeat("x", 20001)+`"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("oversized bulletin = %d", status)
	}
	if status := put(`{"bulletin":"# 欢迎来到 SYUCT OJ"}`, adminCookies); status != http.StatusOK {
		t.Fatalf("admin update = %d", status)
	}

	status, payload = get("/api/v1/settings", nil)
	if status != http.StatusOK || payload.Data.Bulletin != "# 欢迎来到 SYUCT OJ" {
		t.Fatalf("public settings after update = %d %+v", status, payload.Data)
	}
	status, payload = get("/api/v1/admin/settings", adminCookies)
	if status != http.StatusOK || payload.Data.Bulletin != "# 欢迎来到 SYUCT OJ" {
		t.Fatalf("admin settings = %d %+v", status, payload.Data)
	}
}
