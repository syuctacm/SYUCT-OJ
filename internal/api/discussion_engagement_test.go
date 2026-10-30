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

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/message"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestDiscussionViewAndMentionNotifications(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_discussion_engagement_test_%d", time.Now().UnixNano())
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
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	discussionID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": discussionID, "domainId": "system", "docType": 21, "docId": discussionID,
		"title": "通知测试", "content": "正文", "owner": int64(1),
		"nReply": int64(0), "views": int64(1), "parentType": int64(20), "parentId": "qa",
		"updateAt": time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, discussion.NewRepository(db), userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	alice := loginAssetUser(t, server.URL, "alice", "secret")
	bob := loginAssetUser(t, server.URL, "bob", "secret")

	getViews := func(cookies []*http.Cookie) int64 {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/discussions/"+discussionID.Hex(), nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get discussion status = %d", resp.StatusCode)
		}
		var envelope struct {
			Data discussion.Discussion `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data.Views
	}

	if got := getViews(nil); got != 1 {
		t.Fatalf("anonymous views = %d, want 1", got)
	}
	if got := getViews(alice); got != 2 {
		t.Fatalf("alice first view = %d, want 2", got)
	}
	if got := getViews(alice); got != 2 {
		t.Fatalf("alice repeat view = %d, want 2", got)
	}
	if got := getViews(bob); got != 3 {
		t.Fatalf("bob first view = %d, want 3", got)
	}
	var viewStatuses int64
	viewStatuses, err = db.Collection("document.status").CountDocuments(ctx, bson.M{
		"domainId": "system", "docType": 21, "docId": discussionID,
		"uid": bson.M{"$in": []int64{7, 8}}, "view": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if viewStatuses != 2 {
		t.Fatalf("view statuses = %d, want 2", viewStatuses)
	}

	mentionContent := "请 @[](/user/8) 和 @[](/user/8) 看一下，忽略 @[](/user/999) 与 @[](/user/7)"
	status, body := postRawDiscussionReply(t, server.URL, alice, discussionID.Hex(), map[string]string{"content": mentionContent})
	if status != http.StatusCreated {
		t.Fatalf("mention reply status = %d: %s", status, body)
	}
	var replyEnvelope struct {
		Data discussionReply `json:"data"`
	}
	if err := json.Unmarshal(body, &replyEnvelope); err != nil {
		t.Fatal(err)
	}

	var notification struct {
		From    int64   `bson:"from"`
		To      []int64 `bson:"to"`
		Content string  `bson:"content"`
		Flag    int64   `bson:"flag"`
	}
	if err := db.Collection("message").FindOne(ctx, bson.M{"from": int64(1)}).Decode(&notification); err != nil {
		t.Fatal(err)
	}
	if notification.Flag != message.FlagRichtext|message.FlagUnread {
		t.Fatalf("notification flag = %d", notification.Flag)
	}
	if len(notification.To) != 1 || notification.To[0] != 8 {
		t.Fatalf("notification recipients = %+v, want [8]", notification.To)
	}
	if !bytes.Contains([]byte(notification.Content), []byte(`"message":"User {0} mentioned you in {1:link}"`)) ||
		!bytes.Contains([]byte(notification.Content), []byte(`"/discuss/`+discussionID.Hex()+`"`)) {
		t.Fatalf("notification content = %s", notification.Content)
	}
	var bobDoc struct {
		UnreadMsg int64 `bson:"unreadMsg"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(8)}).Decode(&bobDoc); err != nil {
		t.Fatal(err)
	}
	if bobDoc.UnreadMsg != 1 {
		t.Fatalf("bob unread = %d, want 1", bobDoc.UnreadMsg)
	}

	status, body = postRawDiscussionReply(t, server.URL, bob, discussionID.Hex(), map[string]string{
		"content": "楼中楼 @[](/user/7)", "replyTo": replyEnvelope.Data.ID,
	})
	if status != http.StatusCreated {
		t.Fatalf("tail mention reply status = %d: %s", status, body)
	}
	var tailNotification struct {
		To   []int64 `bson:"to"`
		Flag int64   `bson:"flag"`
	}
	if err := db.Collection("message").FindOne(ctx, bson.M{"from": int64(1), "to": int64(7)}).Decode(&tailNotification); err != nil {
		t.Fatal(err)
	}
	if len(tailNotification.To) != 1 || tailNotification.To[0] != 7 {
		t.Fatalf("tail notification recipients = %+v", tailNotification.To)
	}
	var aliceDoc struct {
		UnreadMsg int64 `bson:"unreadMsg"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&aliceDoc); err != nil {
		t.Fatal(err)
	}
	if aliceDoc.UnreadMsg != 1 {
		t.Fatalf("alice unread = %d, want 1", aliceDoc.UnreadMsg)
	}
}

func postRawDiscussionReply(t *testing.T, baseURL string, cookies []*http.Cookie, did string, payload map[string]string) (int, []byte) {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/discussions/"+did+"/replies", bytes.NewReader(body))
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
