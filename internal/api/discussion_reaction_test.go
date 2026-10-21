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
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestDiscussionReactionIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_discussion_reaction_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	discussionID := bson.NewObjectID()
	replyID := bson.NewObjectID()
	otherReplyID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": discussionID, "domainId": "system", "docType": 21, "docId": discussionID,
			"title": "如何训练", "content": "分享一下训练方法", "owner": int64(7), "nReply": int64(1),
			"views": int64(1), "updateAt": time.Now().Add(-time.Hour)},
		bson.M{"_id": replyID, "domainId": "system", "docType": 22, "docId": replyID,
			"parentType": 21, "parentId": discussionID, "owner": int64(7),
			"content": "先刷基础题", "reply": []any{}, "nReply": int64(0)},
		bson.M{"_id": otherReplyID, "domainId": "system", "docType": 22, "docId": otherReplyID,
			"parentType": 21, "parentId": bson.NewObjectID(), "owner": int64(7),
			"content": "别的讨论", "reply": []any{}, "nReply": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	discussionRepo := discussion.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, discussionRepo, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	type reactionResponse struct {
		Data struct {
			Doc struct {
				React map[string]int64 `json:"react"`
			} `json:"doc"`
			Sdoc struct {
				React map[string]int64 `json:"react"`
			} `json:"sdoc"`
		} `json:"data"`
	}

	react := func(nodeType, id, emoji string, reverse bool, cookies []*http.Cookie) (int, reactionResponse) {
		body, _ := json.Marshal(map[string]any{
			"nodeType": nodeType, "id": id, "emoji": emoji, "reverse": reverse,
		})
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/discussions/"+discussionID.Hex()+"/reaction", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var parsed reactionResponse
		if resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, parsed
	}

	if status, _ := react("did", discussionID.Hex(), "👍", false, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous reaction = %d", status)
	}

	cookies := loginDiscussionUser(t, server.URL)

	status, body := react("did", discussionID.Hex(), "👍", false, cookies)
	if status != http.StatusOK || body.Data.Doc.React["👍"] != 1 || body.Data.Sdoc.React["👍"] != 1 {
		t.Fatalf("topic reaction = %d: %+v", status, body.Data)
	}
	// Repeating the same reaction must not double count.
	status, body = react("did", discussionID.Hex(), "👍", false, cookies)
	if status != http.StatusOK || body.Data.Doc.React["👍"] != 1 {
		t.Fatalf("repeated reaction = %d: %+v", status, body.Data)
	}
	status, body = react("did", discussionID.Hex(), "👍", true, cookies)
	if status != http.StatusOK || body.Data.Doc.React["👍"] != 0 {
		t.Fatalf("reverse reaction = %d: %+v", status, body.Data)
	}
	if _, ok := body.Data.Sdoc.React["👍"]; ok {
		t.Fatalf("reaction should be removed from the user status: %+v", body.Data.Sdoc.React)
	}
	var stored struct {
		React map[string]int64 `bson:"react"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": discussionID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.React) != 0 {
		t.Fatalf("zero counters should be cleaned up: %+v", stored.React)
	}
	var raw bson.M
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": discussionID}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["react"]; ok {
		t.Fatalf("empty reaction map should be removed: %+v", raw["react"])
	}

	status, body = react("drid", replyID.Hex(), "🎉", false, cookies)
	if status != http.StatusOK || body.Data.Doc.React["🎉"] != 1 || body.Data.Sdoc.React["🎉"] != 1 {
		t.Fatalf("reply reaction = %d: %+v", status, body.Data)
	}
	if status, _ := react("drid", otherReplyID.Hex(), "🎉", false, cookies); status != http.StatusNotFound {
		t.Fatalf("foreign reply reaction = %d", status)
	}
	if status, _ := react("did", discussionID.Hex(), "a.b", false, cookies); status != http.StatusBadRequest {
		t.Fatalf("invalid emoji = %d", status)
	}
	if status, _ := react("nope", discussionID.Hex(), "👍", false, cookies); status != http.StatusBadRequest {
		t.Fatalf("invalid node type = %d", status)
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/discussions/"+discussionID.Hex(), nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var detail struct {
		Data struct {
			React     map[string]int64            `json:"react"`
			Reactions map[string]map[string]int64 `json:"reactions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.Data.Reactions[replyID.Hex()]["🎉"] != 1 {
		t.Fatalf("detail reactions = %+v", detail.Data.Reactions)
	}
	if _, ok := detail.Data.Reactions[discussionID.Hex()]; ok {
		t.Fatalf("topic reactions were reversed, detail should not report them: %+v", detail.Data.Reactions)
	}
}
