package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func TestUserAndGroupSearchIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_user_search_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(1), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "displayName": "Alice Chen", "avatar": "/avatar/alice.png"},
		bson.M{"_id": int64(2), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com", "mailLower": "bob@example.com", "displayName": "Bob Li"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("user.group").InsertOne(ctx, bson.M{
		"domainId": "system", "name": "2026", "uids": []int64{1, 2},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db, nil, authHandler, nil, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	get := func(path string) (int, []byte) {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		return response.StatusCode, body
	}

	status, body := get("/api/v1/users?search=ali")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"username":"alice"`)) || bytes.Contains(body, []byte("alice@example.com")) {
		t.Fatalf("user search = %d: %s", status, body)
	}
	var userEnvelope struct {
		Data struct {
			Items []struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &userEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(userEnvelope.Data.Items) != 1 || userEnvelope.Data.Items[0].ID != 1 {
		t.Fatalf("user search items: %+v", userEnvelope.Data.Items)
	}

	status, body = get("/api/v1/users?auto=2,1")
	if status != http.StatusOK {
		t.Fatalf("auto users = %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &userEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(userEnvelope.Data.Items) != 2 || userEnvelope.Data.Items[0].ID != 2 || userEnvelope.Data.Items[1].ID != 1 {
		t.Fatalf("auto users order: %+v", userEnvelope.Data.Items)
	}

	status, body = get("/api/v1/groups?search=202")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"name":"2026"`)) || !bytes.Contains(body, []byte(`"uids":[1,2]`)) {
		t.Fatalf("group search = %d: %s", status, body)
	}
	status, body = get("/api/v1/groups?names=2026")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"name":"2026"`)) {
		t.Fatalf("group lookup = %d: %s", status, body)
	}
}
