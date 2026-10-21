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

func TestAdminUsersIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_admin_users_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
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
		bson.M{"_id": int64(5), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756), "avatar": "/avatar/alice.png"},
		bson.M{"_id": int64(6), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com", "mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(7), "uname": "root", "unameLower": "root", "mail": "root@example.com", "mailLower": "root@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(-1)},
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

	type userList struct {
		Data struct {
			Items []struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
				Priv     int64  `json:"priv"`
				Avatar   string `json:"avatar"`
			} `json:"items"`
			Limit int64 `json:"limit"`
		} `json:"data"`
	}

	list := func(query string, cookies []*http.Cookie) (int, userList) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/admin/users"+query, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body userList
		if resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, body
	}

	if status, _ := list("", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous list = %d", status)
	}
	if status, _ := list("", aliceCookies); status != http.StatusForbidden {
		t.Fatalf("non-admin list = %d", status)
	}

	status, page := list("?sort=id&limit=1000", adminCookies)
	if status != http.StatusOK {
		t.Fatalf("admin list = %d", status)
	}
	if page.Data.Limit != 1000 {
		t.Fatalf("limit = %d, want 1000", page.Data.Limit)
	}
	if len(page.Data.Items) != 4 {
		t.Fatalf("items = %+v", page.Data.Items)
	}
	for i := 1; i < len(page.Data.Items); i++ {
		if page.Data.Items[i-1].ID >= page.Data.Items[i].ID {
			t.Fatalf("items are not sorted by id: %+v", page.Data.Items)
		}
	}
	if page.Data.Items[1].Avatar != "/avatar/alice.png" {
		t.Fatalf("avatar missing: %+v", page.Data.Items[1])
	}

	if _, capped := list("?limit=5000", adminCookies); capped.Data.Limit != 1000 {
		t.Fatalf("limit should be capped at 1000, got %d", capped.Data.Limit)
	}

	patch := func(uid int64, priv int64, cookies []*http.Cookie) int {
		body, _ := json.Marshal(map[string]int64{"priv": priv})
		req, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/admin/users/%d/priv", server.URL, uid), bytes.NewReader(body))
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

	if status := patch(5, 4, adminCookies); status != http.StatusOK {
		t.Fatalf("patch priv = %d", status)
	}
	var updated struct {
		Priv int64 `bson:"priv"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(5)}).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Priv != 4 {
		t.Fatalf("priv = %d, want 4", updated.Priv)
	}
	if status := patch(1, 1, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("self patch = %d", status)
	}
	if status := patch(7, 0, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("super admin patch = %d", status)
	}
}
