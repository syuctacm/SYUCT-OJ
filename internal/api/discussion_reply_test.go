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

func TestDiscussionReplyIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_discussion_api_test_%d", time.Now().UnixNano())
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
	discussionID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": discussionID, "domainId": "system", "docType": 21, "docId": discussionID,
		"title": "如何训练", "content": "分享一下训练方法", "owner": int64(1),
		"nReply": int64(0), "views": int64(1), "updateAt": time.Now().Add(-time.Hour),
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

	replyBody := bytes.NewBufferString(`{"content":"先刷基础题"}`)
	resp, err := http.Post(server.URL+"/api/v1/discussions/"+discussionID.Hex()+"/replies", "application/json", replyBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated reply status = %d", resp.StatusCode)
	}

	cookies := loginDiscussionUser(t, server.URL)
	reply := postDiscussionReply(t, server.URL, cookies, discussionID.Hex(), "先刷基础题", http.StatusCreated)
	if reply.Owner != 7 || reply.Content != "先刷基础题" || reply.ID == "" {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if reply.CreatedAt.IsZero() {
		t.Fatal("reply createdAt is zero")
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/discussions/"+discussionID.Hex()+"/replies", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var listEnvelope struct {
		Data struct {
			Items []discussionReply `json:"items"`
			Total int               `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listEnvelope); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || listEnvelope.Data.Total != 1 || len(listEnvelope.Data.Items) != 1 {
		t.Fatalf("reply list status=%d data=%+v", resp.StatusCode, listEnvelope.Data)
	}

	var stored struct {
		NReply   int64     `bson:"nReply"`
		UpdateAt time.Time `bson:"updateAt"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": discussionID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.NReply != 1 || stored.UpdateAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("discussion counters not updated: %+v", stored)
	}

	postDiscussionReply(t, server.URL, cookies, discussionID.Hex(), "   ", http.StatusBadRequest)
	_, err = db.Collection("document").UpdateOne(ctx, bson.M{"_id": discussionID}, bson.M{"$set": bson.M{"lock": true}})
	if err != nil {
		t.Fatal(err)
	}
	postDiscussionReply(t, server.URL, cookies, discussionID.Hex(), "锁定时不能回复", http.StatusConflict)
}

type discussionReply struct {
	ID        string    `json:"id"`
	Owner     int64     `json:"owner"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func loginDiscussionUser(t *testing.T, baseURL string) []*http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "secret"})
	resp, err := http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	return resp.Cookies()
}

func postDiscussionReply(t *testing.T, baseURL string, cookies []*http.Cookie, did, content string, wantStatus int) discussionReply {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"content": content})
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
	if resp.StatusCode != wantStatus {
		var payload any
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		t.Fatalf("reply status = %d, want %d, payload=%+v", resp.StatusCode, wantStatus, payload)
	}
	if wantStatus != http.StatusCreated {
		return discussionReply{}
	}
	var envelope struct {
		Data discussionReply `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
