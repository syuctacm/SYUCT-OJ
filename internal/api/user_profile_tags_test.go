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

func TestUserProfileAcceptedProblemTags(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_profile_tags_test_%d", time.Now().UnixNano())
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
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(101), "pid": "P101", "title": "DP", "tag": []string{"dp", "math"}, "hidden": false},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(102), "pid": "P102", "title": "DP 2", "tag": []string{"dp"}, "hidden": false},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(103), "pid": "P103", "title": "Hidden", "tag": []string{"hidden"}, "hidden": true},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(104), "pid": "P104", "title": "Wrong", "tag": []string{"wrong"}, "hidden": false},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "uid": int64(7), "pid": int64(101), "status": 1},
		bson.M{"domainId": "system", "uid": int64(7), "pid": int64(101), "status": 1},
		bson.M{"domainId": "system", "uid": int64(7), "pid": int64(102), "status": 1},
		bson.M{"domainId": "system", "uid": int64(7), "pid": int64(103), "status": 1},
		bson.M{"domainId": "system", "uid": int64(7), "pid": int64(104), "status": 2},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	router := api.NewRouter(config.Config{}, db, nil, nil, nil, nil, nil, nil, nil, userRepo, nil, nil)
	server := httptest.NewServer(router)
	defer server.Close()

	getTags := func(uid int64) []struct {
		Tag   string `json:"tag"`
		Count int64  `json:"count"`
	} {
		t.Helper()
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/users/%d", server.URL, uid))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("profile status = %d", resp.StatusCode)
		}
		var envelope struct {
			Data struct {
				Tags []struct {
					Tag   string `json:"tag"`
					Count int64  `json:"count"`
				} `json:"tags"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data.Tags
	}

	tags := getTags(7)
	got := map[string]int64{}
	for _, item := range tags {
		got[item.Tag] = item.Count
	}
	if len(tags) != 2 || got["dp"] != 2 || got["math"] != 1 {
		t.Fatalf("tags = %+v", tags)
	}
	if _, ok := got["hidden"]; ok {
		t.Fatalf("hidden problem tag leaked: %+v", tags)
	}
	if _, ok := got["wrong"]; ok {
		t.Fatalf("unaccepted problem tag leaked: %+v", tags)
	}
	if empty := getTags(8); len(empty) != 0 {
		t.Fatalf("empty tags = %+v", empty)
	}
}
