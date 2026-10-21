package api_test

import (
	"bytes"
	"context"
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

func TestHomeMessagesIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_messages_test_%d", time.Now().UnixNano())
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
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	alice := loginAssetUser(t, server.URL, "alice", "secret")
	bob := loginAssetUser(t, server.URL, "bob", "secret")

	call := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
		var reader io.Reader
		if body != "" {
			reader = bytes.NewBufferString(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, reader)
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}

	if status, _ := call(http.MethodGet, "/api/v1/messages", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous list = %d", status)
	}
	if status, _ := call(http.MethodPost, "/api/v1/messages", `{"uid":8,"content":"hi"}`, alice); status != http.StatusCreated {
		t.Fatalf("send = %d", status)
	}
	if status, _ := call(http.MethodPost, "/api/v1/messages", `{"uid":999,"content":"hi"}`, alice); status != http.StatusNotFound {
		t.Fatalf("send to missing user = %d", status)
	}
	if status, _ := call(http.MethodPost, "/api/v1/messages", `{"uid":8,"content":"  "}`, alice); status != http.StatusBadRequest {
		t.Fatalf("send empty = %d", status)
	}

	var bobDoc struct {
		UnreadMsg int64 `bson:"unreadMsg"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(8)}).Decode(&bobDoc); err != nil {
		t.Fatal(err)
	}
	if bobDoc.UnreadMsg != 1 {
		t.Fatalf("bob unread after send = %d", bobDoc.UnreadMsg)
	}

	status, body := call(http.MethodGet, "/api/v1/messages", "", bob)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"username":"alice"`)) || !bytes.Contains(body, []byte(`"content":"hi"`)) {
		t.Fatalf("bob dialogues = %d: %s", status, body)
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(8)}).Decode(&bobDoc); err != nil {
		t.Fatal(err)
	}
	if bobDoc.UnreadMsg != 0 {
		t.Fatalf("bob unread after read = %d", bobDoc.UnreadMsg)
	}

	status, body = call(http.MethodGet, "/api/v1/messages", "", alice)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"username":"bob"`)) {
		t.Fatalf("alice dialogues = %d: %s", status, body)
	}

	var messageDoc struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := db.Collection("message").FindOne(ctx, bson.M{"from": int64(7)}).Decode(&messageDoc); err != nil {
		t.Fatal(err)
	}
	if status, _ := call(http.MethodDelete, "/api/v1/messages/"+messageDoc.ID.Hex(), "", bob); status != http.StatusNotFound {
		t.Fatalf("bob delete alice message = %d", status)
	}
	if status, _ := call(http.MethodDelete, "/api/v1/messages/"+messageDoc.ID.Hex(), "", alice); status != http.StatusNoContent {
		t.Fatalf("alice delete own message = %d", status)
	}
	if status, body := call(http.MethodGet, "/api/v1/messages", "", bob); status != http.StatusOK || bytes.Contains(body, []byte(`"content":"hi"`)) {
		t.Fatalf("bob dialogues after delete = %d: %s", status, body)
	}
}
