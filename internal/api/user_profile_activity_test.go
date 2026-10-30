package api_test

import (
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
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestUserRecentContests(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_profile_activity_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "priv": user.DefaultPriv},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "priv": user.DefaultPriv},
	}); err != nil {
		t.Fatal(err)
	}
	contestID := bson.NewObjectID()
	homeworkID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 30, "docId": contestID,
			"title": "ACM 周赛", "rule": "acm", "beginAt": time.Now().Add(-time.Hour), "endAt": time.Now().Add(time.Hour)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 30, "docId": homeworkID,
			"title": "第一周作业", "rule": "homework", "beginAt": time.Now().Add(-time.Hour), "endAt": time.Now().Add(time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document.status").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "docType": 30, "docId": contestID, "uid": int64(7), "attend": 1},
		bson.M{"domainId": "system", "docType": 30, "docId": homeworkID, "uid": int64(7), "attend": 1},
		bson.M{"domainId": "system", "docType": 30, "docId": bson.NewObjectID(), "uid": int64(8), "attend": 1},
	}); err != nil {
		t.Fatal(err)
	}
	userRepo := user.NewRepository(db)
	router := api.NewRouter(config.Config{}, db, nil, nil, nil, nil, nil, nil, nil, userRepo, nil, nil)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/users/7")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("profile status = %d", resp.StatusCode)
	}
	var envelope struct {
		Data struct {
			Tdocs []struct {
				DocID string `json:"docId"`
				Title string `json:"title"`
				Rule  string `json:"rule"`
			} `json:"tdocs"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Tdocs) != 2 {
		t.Fatalf("recent contests = %+v", envelope.Data.Tdocs)
	}
	got := map[string]string{}
	for _, item := range envelope.Data.Tdocs {
		got[item.Title] = item.Rule
	}
	if got["ACM 周赛"] != "acm" || got["第一周作业"] != "homework" {
		t.Fatalf("recent contest rules = %+v", got)
	}
}
