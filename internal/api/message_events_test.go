package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/message"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestMessageEventStream(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_message_events_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "system", "unameLower": "system", "displayName": "系统消息", "priv": int64(0)},
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	discussionID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 20, "docId": "qa", "title": "问答", "content": "qa", "owner": int64(1)},
		bson.M{"_id": discussionID, "domainId": "system", "docType": 21, "docId": discussionID, "title": "提及事件", "content": "正文",
			"owner": int64(7), "parentType": 20, "parentId": "qa", "nReply": 0, "views": 0, "updateAt": time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{}, db, nil, authHandler, nil, nil, nil, nil,
		discussion.NewRepository(db), userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	alice := loginAssetUser(t, server.URL, "alice", "secret")
	bob := loginAssetUser(t, server.URL, "bob", "secret")
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/messages/events"
	header := http.Header{}
	header.Set("Cookie", cookieHeader(bob))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial message events: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	if _, _, err := websocket.DefaultDialer.Dial(wsURL, nil); err == nil {
		t.Fatal("anonymous websocket unexpectedly connected")
	}

	call := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
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
		data := new(bytes.Buffer)
		_, _ = data.ReadFrom(resp.Body)
		return resp.StatusCode, data.Bytes()
	}

	status, body := call(http.MethodPost, "/api/v1/messages", `{"uid":8,"content":"实时消息"}`, alice)
	if status != http.StatusCreated {
		t.Fatalf("send message = %d: %s", status, body)
	}
	readMessageEvent(t, conn, func(event message.Message) bool {
		return event.From == 7 && event.Content == "实时消息"
	})

	status, body = postRawDiscussionReply(t, server.URL, alice, discussionID.Hex(), map[string]string{
		"content": "提醒一下 @[](/user/8)",
	})
	if status != http.StatusCreated {
		t.Fatalf("mention reply = %d: %s", status, body)
	}
	readMessageEvent(t, conn, func(event message.Message) bool {
		return event.From == 1 && strings.Contains(event.Content, "mentioned you")
	})
}

func readMessageEvent(t *testing.T, conn *websocket.Conn, matches func(message.Message) bool) {
	t.Helper()
	for attempt := 0; attempt < 3; attempt++ {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read message event: %v", err)
		}
		var event message.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode message event: %v", err)
		}
		if matches(event.Message) {
			return
		}
	}
	t.Fatal("expected message event not received")
}
