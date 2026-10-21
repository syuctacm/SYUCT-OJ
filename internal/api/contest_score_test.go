package api_test

import (
	"bytes"
	"context"
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
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestScoreIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_contest_score_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com", "mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com", "mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	contestID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID,
		"title": "OI 比赛", "rule": "oi", "owner": int64(7), "pids": []int64{1001, 1002},
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(8), "pid": int64(1001), "status": 1, "score": 60, "contest": contestID},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(8), "pid": int64(1002), "status": 1, "score": 100, "contest": contestID},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, contestRepo, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")
	bobCookies := loginAssetUser(t, server.URL, "bob", "secret")

	post := func(body string, cookies []*http.Cookie) int {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests/"+contestID.Hex()+"/score", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	if status := post(`{"pid":1001,"score":80}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous score = %d", status)
	}
	if status := post(`{"pid":1001,"score":80}`, bobCookies); status != http.StatusForbidden {
		t.Fatalf("non-editor score = %d", status)
	}
	if status := post(`{"pid":9999,"score":80}`, aliceCookies); status != http.StatusNotFound {
		t.Fatalf("unknown pid = %d", status)
	}
	if status := post(`{"pid":1001,"score":-1}`, aliceCookies); status != http.StatusBadRequest {
		t.Fatalf("negative score = %d", status)
	}
	if status := post(`{"pid":1001,"score":20000}`, aliceCookies); status != http.StatusBadRequest {
		t.Fatalf("huge score = %d", status)
	}
	if status := post(`{"pid":1001,"score":80}`, aliceCookies); status != http.StatusNoContent {
		t.Fatalf("owner score = %d", status)
	}

	var doc struct {
		Score map[string]int `bson:"score"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": contestID}).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Score["1001"] != 80 {
		t.Fatalf("stored score = %+v", doc.Score)
	}

	type scoreboardResponse struct {
		Data struct {
			Rows []struct {
				Problems map[string]struct {
					Score  int  `json:"score"`
					Solved bool `json:"solved"`
				} `json:"problems"`
			} `json:"rows"`
		} `json:"data"`
	}
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/contests/"+contestID.Hex()+"/scoreboard", nil)
	for _, cookie := range aliceCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var board scoreboardResponse
	if err := decodeJSON(resp.Body, &board); err != nil {
		t.Fatal(err)
	}
	if len(board.Data.Rows) != 1 {
		t.Fatalf("scoreboard rows = %+v", board.Data.Rows)
	}
	cells := board.Data.Rows[0].Problems
	if cells["1001"].Score != 60 || !cells["1001"].Solved || cells["1002"].Score != 100 {
		t.Fatalf("scoreboard cells = %+v", cells)
	}
}
