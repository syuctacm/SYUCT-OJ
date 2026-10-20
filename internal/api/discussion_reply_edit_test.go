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
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestDiscussionReplyEditDeleteIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_discussion_reply_edit_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "owner", "unameLower": "owner", "mail": "owner@example.com", "mailLower": "owner@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(8), "uname": "author", "unameLower": "author", "mail": "author@example.com", "mailLower": "author@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(9), "uname": "tail", "unameLower": "tail", "mail": "tail@example.com", "mailLower": "tail@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(10), "uname": "other", "unameLower": "other", "mail": "other@example.com", "mailLower": "other@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	did := bson.NewObjectID()
	replyID := bson.NewObjectID()
	tailID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": did, "domainId": "system", "docType": 21, "docId": did, "title": "讨论", "content": "正文", "owner": int64(7), "nReply": int64(2), "views": int64(1), "parentType": int64(20), "parentId": "qa", "updateAt": time.Now()},
		bson.M{
			"_id": replyID, "domainId": "system", "docType": 22, "docId": replyID,
			"parentType": int64(21), "parentId": did, "owner": int64(8), "content": "主回复", "edited": false,
			"reply": []any{
				bson.M{"_id": tailID, "owner": int64(9), "content": "楼中楼", "edited": false},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	discussionRepo := discussion.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db, nil, authHandler, nil, nil, nil, nil, discussionRepo, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	ownerCookies := loginAssetUser(t, server.URL, "owner", "secret")
	authorCookies := loginAssetUser(t, server.URL, "author", "secret")
	tailCookies := loginAssetUser(t, server.URL, "tail", "secret")
	otherCookies := loginAssetUser(t, server.URL, "other", "secret")

	do := func(method, path string, payload any, cookies []*http.Cookie) (int, []byte) {
		var reader io.Reader
		if payload != nil {
			body, _ := json.Marshal(payload)
			reader = bytes.NewReader(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, reader)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode, body
	}

	status, _ := do(http.MethodPatch, "/api/v1/discussions/"+did.Hex()+"/replies/"+replyID.Hex(), map[string]any{"content": "x"}, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated reply edit = %d", status)
	}
	status, _ = do(http.MethodPatch, "/api/v1/discussions/"+did.Hex()+"/replies/"+replyID.Hex(), map[string]any{"content": "x"}, otherCookies)
	if status != http.StatusForbidden {
		t.Fatalf("non-author reply edit = %d", status)
	}
	status, body := do(http.MethodPatch, "/api/v1/discussions/"+did.Hex()+"/replies/"+replyID.Hex(), map[string]any{"content": "主回复已更新"}, authorCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"edited":true`)) || !bytes.Contains(body, []byte("主回复已更新")) {
		t.Fatalf("author reply edit = %d: %s", status, body)
	}
	status, body = do(http.MethodPatch, "/api/v1/discussions/"+did.Hex()+"/replies/"+tailID.Hex(), map[string]any{"content": "楼中楼已更新"}, adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte("楼中楼已更新")) {
		t.Fatalf("admin tail edit = %d: %s", status, body)
	}
	status, _ = do(http.MethodDelete, "/api/v1/discussions/"+did.Hex()+"/replies/"+tailID.Hex(), nil, otherCookies)
	if status != http.StatusForbidden {
		t.Fatalf("non-owner tail delete = %d", status)
	}
	status, _ = do(http.MethodDelete, "/api/v1/discussions/"+did.Hex()+"/replies/"+tailID.Hex(), nil, tailCookies)
	if status != http.StatusNoContent {
		t.Fatalf("tail delete = %d", status)
	}
	status, _ = do(http.MethodDelete, "/api/v1/discussions/"+did.Hex()+"/replies/"+replyID.Hex(), nil, ownerCookies)
	if status != http.StatusNoContent {
		t.Fatalf("topic owner main reply delete = %d", status)
	}

	var discussionDoc struct {
		NReply int64 `bson:"nReply"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": did}).Decode(&discussionDoc); err != nil {
		t.Fatal(err)
	}
	if discussionDoc.NReply != 1 {
		t.Fatalf("nReply = %d, want 1", discussionDoc.NReply)
	}
	count, err := db.Collection("document").CountDocuments(ctx, bson.M{"_id": replyID, "docType": 22})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("main reply still exists: %d", count)
	}
}
