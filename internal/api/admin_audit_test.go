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

func TestAdminAuditLog(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_admin_audit_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv},
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
	cookies := loginAssetUser(t, server.URL, "admin", "secret")

	call := func(method, path, body string) (int, []byte) {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		payload := new(bytes.Buffer)
		_, _ = payload.ReadFrom(resp.Body)
		return resp.StatusCode, payload.Bytes()
	}

	if status, body := call(http.MethodPatch, "/api/v1/admin/users/7/priv", `{"priv":4}`); status != http.StatusOK {
		t.Fatalf("patch priv = %d: %s", status, body)
	}
	if status, body := call(http.MethodPatch, "/api/v1/admin/users/7/ban", `{"banned":true}`); status != http.StatusOK {
		t.Fatalf("ban = %d: %s", status, body)
	}
	status, body := call(http.MethodGet, "/api/v1/admin/audit?limit=10", "")
	if status != http.StatusOK {
		t.Fatalf("audit = %d: %s", status, body)
	}
	var envelope struct {
		Data struct {
			Items []struct {
				Action     string         `json:"action"`
				ActorName  string         `json:"actorName"`
				TargetName string         `json:"targetName"`
				Details    map[string]any `json:"details"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Items) < 2 {
		t.Fatalf("audit items = %+v", envelope.Data.Items)
	}
	seen := map[string]bool{}
	for _, item := range envelope.Data.Items {
		seen[item.Action] = true
		if item.ActorName != "admin" || item.TargetName != "alice" {
			t.Fatalf("audit names = %+v", item)
		}
	}
	if !seen["user.priv"] || !seen["user.ban"] {
		t.Fatalf("audit actions = %+v", seen)
	}
}
