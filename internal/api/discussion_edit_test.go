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

func TestDiscussionEditDeleteLockIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_discussion_edit_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(8), "uname": "other", "unameLower": "other", "mail": "other@example.com", "mailLower": "other@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	nodeID := "qa"
	did := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": nodeID, "domainId": "system", "docType": 20, "docId": nodeID, "title": "问答", "content": "探索"},
		bson.M{"_id": did, "domainId": "system", "docType": 21, "docId": did, "title": "原题", "content": "原内容", "owner": int64(7), "nReply": int64(1), "views": int64(3), "parentType": int64(20), "parentId": nodeID, "updateAt": time.Now().Add(-time.Hour)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 22, "docId": bson.NewObjectID(), "parentType": int64(21), "parentId": did, "owner": int64(8), "content": "回复"},
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

	status, _ := do(http.MethodPatch, "/api/v1/discussions/"+did.Hex(), map[string]any{"title": "未登录", "content": "x"}, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated update = %d", status)
	}

	status, body := do(http.MethodPatch, "/api/v1/discussions/"+did.Hex(), map[string]any{
		"title": "新标题", "content": "新内容", "pin": false, "highlight": false,
	}, ownerCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"edited":true`)) || !bytes.Contains(body, []byte("新标题")) {
		t.Fatalf("owner update = %d: %s", status, body)
	}

	status, _ = do(http.MethodPatch, "/api/v1/discussions/"+did.Hex(), map[string]any{
		"title": "越权", "content": "x", "pin": true, "highlight": false,
	}, otherCookies)
	if status != http.StatusForbidden {
		t.Fatalf("non-owner update = %d", status)
	}

	status, body = do(http.MethodPatch, "/api/v1/discussions/"+did.Hex(), map[string]any{
		"title": "管理员编辑", "content": "管理员内容", "pin": true, "highlight": true,
	}, adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"pin":true`)) || !bytes.Contains(body, []byte(`"highlight":true`)) {
		t.Fatalf("admin update = %d: %s", status, body)
	}

	status, _ = do(http.MethodPatch, "/api/v1/discussions/"+did.Hex()+"/lock", map[string]any{"lock": true}, ownerCookies)
	if status != http.StatusForbidden {
		t.Fatalf("owner lock = %d", status)
	}
	status, body = do(http.MethodPatch, "/api/v1/discussions/"+did.Hex()+"/lock", map[string]any{"lock": true}, adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"lock":true`)) {
		t.Fatalf("admin lock = %d: %s", status, body)
	}

	status, _ = do(http.MethodPost, "/api/v1/discussions/"+did.Hex()+"/replies", map[string]any{"content": "锁定回复"}, otherCookies)
	if status != http.StatusConflict {
		t.Fatalf("locked reply = %d", status)
	}

	status, _ = do(http.MethodDelete, "/api/v1/discussions/"+did.Hex(), nil, otherCookies)
	if status != http.StatusForbidden {
		t.Fatalf("non-owner delete = %d", status)
	}
	status, _ = do(http.MethodDelete, "/api/v1/discussions/"+did.Hex(), nil, ownerCookies)
	if status != http.StatusNoContent {
		t.Fatalf("owner delete = %d", status)
	}
	topicCount, err := db.Collection("document").CountDocuments(ctx, bson.M{"_id": did, "docType": 21})
	if err != nil {
		t.Fatal(err)
	}
	replyCount, err := db.Collection("document").CountDocuments(ctx, bson.M{"parentId": did, "docType": 22})
	if err != nil {
		t.Fatal(err)
	}
	if topicCount != 0 || replyCount != 0 {
		t.Fatalf("delete left topic=%d reply=%d", topicCount, replyCount)
	}
}
