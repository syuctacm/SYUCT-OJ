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

func TestDiscussionCreateIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_discussion_create_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	_, err = db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": "qa", "domainId": "system", "docType": 20, "docId": "qa", "title": "问答", "content": "探索", "owner": int64(1)},
		bson.M{"_id": "share", "domainId": "system", "docType": 20, "docId": "share", "title": "分享", "content": "探索", "owner": int64(1)},
	})
	if err != nil {
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

	resp, err := http.Get(server.URL + "/api/v1/discussion-nodes")
	if err != nil {
		t.Fatal(err)
	}
	var nodes struct {
		Data struct {
			Items []struct {
				DocID    string `json:"docId"`
				Title    string `json:"title"`
				Category string `json:"category"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(nodes.Data.Items) != 2 || nodes.Data.Items[0].DocID != "qa" || nodes.Data.Items[0].Category != "探索" {
		t.Fatalf("unexpected nodes status=%d data=%+v", resp.StatusCode, nodes.Data)
	}

	createBody, _ := json.Marshal(map[string]any{"nodeId": "qa", "title": "如何开始训练", "content": "请给出训练路线"})
	resp, err = http.Post(server.URL+"/api/v1/discussions", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create status = %d", resp.StatusCode)
	}

	cookies := loginDiscussionUser(t, server.URL)
	postDiscussion(t, server.URL, cookies, map[string]any{"nodeId": "missing", "title": "标题", "content": "正文"}, http.StatusBadRequest)
	postDiscussion(t, server.URL, cookies, map[string]any{"nodeId": "qa", "title": "  ", "content": "正文"}, http.StatusBadRequest)
	created := postDiscussion(t, server.URL, cookies, map[string]any{
		"nodeId": "qa", "title": "如何开始训练", "content": "请给出**训练路线**", "pin": true, "highlight": true,
	}, http.StatusCreated)
	if created.Title != "如何开始训练" || created.ParentType != 20 || created.ParentID != "qa" || created.ParentTitle != "问答" || !created.Pin || !created.Highlight {
		t.Fatalf("unexpected discussion: %+v", created)
	}

	var stored struct {
		DocType    int    `bson:"docType"`
		ParentType int    `bson:"parentType"`
		ParentID   string `bson:"parentId"`
		Owner      int64  `bson:"owner"`
		NReply     int64  `bson:"nReply"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": mustObjectID(t, created.ID)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.DocType != 21 || stored.ParentType != 20 || stored.ParentID != "qa" || stored.Owner != 7 || stored.NReply != 0 {
		t.Fatalf("unexpected stored discussion: %+v", stored)
	}
}

type createdDiscussion struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ParentType  int    `json:"parentType"`
	ParentID    string `json:"parentId"`
	ParentTitle string `json:"parentTitle"`
	Pin         bool   `json:"pin"`
	Highlight   bool   `json:"highlight"`
}

func postDiscussion(t *testing.T, baseURL string, cookies []*http.Cookie, payload map[string]any, wantStatus int) createdDiscussion {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/discussions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		var data any
		_ = json.NewDecoder(resp.Body).Decode(&data)
		t.Fatalf("create status = %d, want %d, data=%+v", resp.StatusCode, wantStatus, data)
	}
	if wantStatus != http.StatusCreated {
		return createdDiscussion{}
	}
	var envelope struct {
		Data createdDiscussion `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func mustObjectID(t *testing.T, value string) bson.ObjectID {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(value)
	if err != nil {
		t.Fatal(err)
	}
	return oid
}
