package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestAdminUserBanRevokesAccess(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_admin_ban_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
			"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(5), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv},
		bson.M{"_id": int64(6), "uname": "root", "unameLower": "root", "mail": "root@example.com",
			"mailLower": "root@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(-1)},
	}); err != nil {
		t.Fatal(err)
	}
	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	handler.NewAdminHandler(db, userRepo, sessions).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")

	ban := func(uid int64, banned bool, cookies []*http.Cookie) int {
		body, _ := json.Marshal(map[string]bool{"banned": banned})
		req, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/admin/users/%d/ban", server.URL, uid), bytes.NewReader(body))
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

	if status := ban(5, true, adminCookies); status != http.StatusOK {
		t.Fatalf("ban = %d", status)
	}
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/me", nil)
	for _, cookie := range aliceCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("banned me = %d", resp.StatusCode)
	}
	if status, _ := postJSON(t, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "secret",
	}, nil); status != http.StatusForbidden {
		t.Fatalf("banned login = %d", status)
	}
	if status := ban(5, false, adminCookies); status != http.StatusOK {
		t.Fatalf("unban = %d", status)
	}
	if status, _ := postJSON(t, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "secret",
	}, nil); status != http.StatusOK {
		t.Fatalf("unbanned login = %d", status)
	}
	if status := ban(1, true, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("self ban = %d", status)
	}
	if status := ban(6, true, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("super admin ban = %d", status)
	}
}
