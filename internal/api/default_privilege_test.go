package api_test

import (
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

func TestDefaultPrivilegeIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_default_privilege_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	customPriv := int64(4 | 1<<7)
	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
			"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(7), "uname": "default_user", "unameLower": "default_user", "mail": "default@example.com",
			"mailLower": "default@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv},
		bson.M{"_id": int64(8), "uname": "custom_user", "unameLower": "custom_user", "mail": "custom@example.com",
			"mailLower": "custom@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": customPriv},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	adminHandler.RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	admin := loginAssetUser(t, server.URL, "admin", "secret")
	normal := loginAssetUser(t, server.URL, "default_user", "secret")

	status, body := getJSON(t, server.URL+"/api/v1/admin/default-priv", admin)
	if status != http.StatusOK {
		t.Fatalf("get default privilege = %d: %s", status, body)
	}
	var response struct {
		Data struct {
			Priv int64 `json:"priv"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Data.Priv != user.DefaultPriv {
		t.Fatalf("default privilege payload = %s (%v)", body, err)
	}

	status, body = rawRequest(t, http.MethodPut, server.URL+"/api/v1/admin/default-priv", []byte(`{"priv":4}`), normal)
	if status != http.StatusForbidden {
		t.Fatalf("non-admin update = %d: %s", status, body)
	}
	status, body = rawRequest(t, http.MethodPut, server.URL+"/api/v1/admin/default-priv", []byte(`{"priv":-1}`), admin)
	if status != http.StatusBadRequest {
		t.Fatalf("negative default privilege = %d: %s", status, body)
	}

	nextPriv := int64(4 | 1<<16)
	status, body = rawRequest(t, http.MethodPut, server.URL+"/api/v1/admin/default-priv",
		[]byte(fmt.Sprintf(`{"priv":%d}`, nextPriv)), admin)
	if status != http.StatusOK {
		t.Fatalf("update default privilege = %d: %s", status, body)
	}

	var accounts []user.User
	cursor, err := db.Collection("user").Find(ctx, bson.M{"_id": bson.M{"$in": []int64{7, 8}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := cursor.All(ctx, &accounts); err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		switch account.ID {
		case 7:
			if account.Priv != nextPriv {
				t.Fatalf("default user privilege = %d", account.Priv)
			}
		case 8:
			if account.Priv != customPriv {
				t.Fatalf("custom user privilege changed = %d", account.Priv)
			}
		}
	}

	newUser, err := userRepo.Create(ctx, user.RegisterInput{
		Mail: "new@example.com", Username: "new_user", Password: "secret", IP: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if newUser.Priv != nextPriv {
		t.Fatalf("new user privilege = %d, want %d", newUser.Priv, nextPriv)
	}

	count, err := db.Collection("oplog").CountDocuments(ctx, bson.M{"action": "admin.default_priv"})
	if err != nil || count != 1 {
		t.Fatalf("default privilege audit count = %d, err=%v", count, err)
	}
}
