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

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestUserProfilePresenceAndWechat(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_profile_presence_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro",
			"priv": int64(0), "wechat": "wx-alice", "school": "SYUCT", "studentId": "20250001"},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions, err := auth.NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatalf("session manager: %v", err)
	}
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{}, db, nil, authHandler, nil, nil, nil, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	// Alice has a browser session; Bob has not logged in.
	_ = loginAssetUser(t, server.URL, "alice", "secret")

	getProfile := func(uid int64) map[string]any {
		t.Helper()
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/users/%d", server.URL, uid))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("profile status = %d", resp.StatusCode)
		}
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}

	alice := getProfile(7)
	if alice["wechat"] != "wx-alice" {
		t.Fatalf("wechat = %#v", alice["wechat"])
	}
	if _, ok := alice["lastSeenAt"].(string); !ok {
		t.Fatalf("lastSeenAt missing: %#v", alice["lastSeenAt"])
	}
	for _, private := range []string{"school", "studentId", "email"} {
		if _, ok := alice[private]; ok {
			t.Fatalf("anonymous profile leaked %s", private)
		}
	}

	bob := getProfile(8)
	if _, ok := bob["lastSeenAt"]; ok {
		t.Fatalf("offline user unexpectedly has lastSeenAt: %#v", bob["lastSeenAt"])
	}
}
