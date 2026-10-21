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
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestBalloonIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_contest_balloon_test_%d", time.Now().UnixNano())
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
	recordID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID,
		"title": "气球比赛", "rule": "acm", "owner": int64(1), "pids": []int64{1001, 1002},
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "balloon": bson.M{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{
		"_id": recordID, "domainId": "system", "uid": int64(7), "pid": int64(1001),
		"status": 0, "score": 0, "lang": "cpp", "contest": contestID,
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	submissionRepo.SetAcceptedHook(func(ctx context.Context, cid bson.ObjectID, uid, pid int64) {
		_ = contestRepo.AddBalloon(ctx, cid, uid, pid)
	})
	if err := submissionRepo.ApplyJudgeResult(ctx, map[string]any{
		"rid": recordID.Hex(), "status": float64(1), "score": float64(100),
	}); err != nil {
		t.Fatalf("apply judge result: %v", err)
	}
	var balloonCount int64
	if balloonCount, err = db.Collection("contest.balloon").CountDocuments(ctx, bson.M{"docId": contestID}); err != nil {
		t.Fatal(err)
	}
	if balloonCount != 1 {
		t.Fatalf("balloon should be created on accept, got %d", balloonCount)
	}
	// A second accepted submission for the same user and problem must not duplicate it.
	if err := submissionRepo.ApplyJudgeResult(ctx, map[string]any{
		"rid": recordID.Hex(), "status": float64(1), "score": float64(100),
	}); err != nil {
		t.Fatal(err)
	}
	if balloonCount, err = db.Collection("contest.balloon").CountDocuments(ctx, bson.M{"docId": contestID}); err != nil {
		t.Fatal(err)
	}
	if balloonCount != 1 {
		t.Fatalf("balloon should not duplicate, got %d", balloonCount)
	}

	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, contestRepo, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")

	do := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
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
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}

	if status, _ := do(http.MethodGet, "/api/v1/contests/"+contestID.Hex()+"/balloons", "", aliceCookies); status != http.StatusForbidden {
		t.Fatalf("non-editor balloons = %d", status)
	}
	status, body := do(http.MethodGet, "/api/v1/contests/"+contestID.Hex()+"/balloons", "", adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"pid":1001`)) {
		t.Fatalf("balloons = %d: %s", status, body)
	}

	colors := `{"colors":{"1001":{"name":"红","color":"#ff0000"},"1002":{"name":"蓝","color":"#0000ff"}}}`
	if status, body := do(http.MethodPut, "/api/v1/contests/"+contestID.Hex()+"/balloons/colors", colors, adminCookies); status != http.StatusNoContent {
		t.Fatalf("colors = %d: %s", status, body)
	}
	var stored struct {
		Balloon map[string]struct {
			Name  string `bson:"name"`
			Color string `bson:"color"`
		} `bson:"balloon"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": contestID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Balloon["1001"].Name != "红" || stored.Balloon["1002"].Color != "#0000ff" {
		t.Fatalf("stored colors = %+v", stored.Balloon)
	}

	var balloon struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := db.Collection("contest.balloon").FindOne(ctx, bson.M{"docId": contestID}).Decode(&balloon); err != nil {
		t.Fatal(err)
	}
	if status, body := do(http.MethodPost, "/api/v1/contests/"+contestID.Hex()+"/balloons/"+balloon.ID.Hex()+"/sent", "", adminCookies); status != http.StatusNoContent {
		t.Fatalf("mark sent = %d: %s", status, body)
	}
	if status, _ := do(http.MethodPost, "/api/v1/contests/"+contestID.Hex()+"/balloons/"+balloon.ID.Hex()+"/sent", "", adminCookies); status != http.StatusNotFound {
		t.Fatalf("double mark sent = %d", status)
	}
	var sent struct {
		Sent int64 `bson:"sent"`
	}
	if err := db.Collection("contest.balloon").FindOne(ctx, bson.M{"_id": balloon.ID}).Decode(&sent); err != nil {
		t.Fatal(err)
	}
	if sent.Sent != 1 {
		t.Fatalf("sent = %d", sent.Sent)
	}
}
