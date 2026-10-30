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
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestAssignVisibilityAndAccess(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_assign_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv},
		bson.M{"_id": int64(8), "uname": "admin", "unameLower": "admin", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(9), "uname": "owner", "unameLower": "owner", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("user.group").InsertOne(ctx, bson.M{
		"domainId": "system", "name": "2026", "uids": []int64{7},
	}); err != nil {
		t.Fatal(err)
	}

	publicID := bson.NewObjectID()
	groupID := bson.NewObjectID()
	otherID := bson.NewObjectID()
	ownerID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		contestAssignDoc(publicID, "公开赛", 9, nil),
		contestAssignDoc(groupID, "新生赛", 9, []string{"2026"}),
		contestAssignDoc(otherID, "限定赛", 9, []string{"2025"}),
		contestAssignDoc(ownerID, "我的比赛", 9, []string{"2025"}),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions, err := auth.NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{}, db, nil, authHandler, nil, nil, contestRepo, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	list := func(cookies []*http.Cookie) (int, []string, []string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/contests", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var payload struct {
			Data struct {
				Items []struct {
					Title string `json:"title"`
				} `json:"items"`
				Groups []string `json:"groups"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		titles := make([]string, 0, len(payload.Data.Items))
		for _, item := range payload.Data.Items {
			titles = append(titles, item.Title)
		}
		return resp.StatusCode, titles, payload.Data.Groups
	}
	get := func(cookies []*http.Cookie, id string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/contests/"+id, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	status, titles, groups := list(nil)
	if status != http.StatusOK || len(titles) != 1 || titles[0] != "公开赛" || len(groups) != 0 {
		t.Fatalf("anonymous list = %d %+v groups=%+v", status, titles, groups)
	}
	if code := get(nil, groupID.Hex()); code != http.StatusForbidden {
		t.Fatalf("anonymous assigned contest = %d", code)
	}

	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")
	status, titles, groups = list(aliceCookies)
	if status != http.StatusOK || len(titles) != 2 || len(groups) != 1 || groups[0] != "2026" {
		t.Fatalf("alice list = %d %+v groups=%+v", status, titles, groups)
	}
	if code := get(aliceCookies, groupID.Hex()); code != http.StatusOK {
		t.Fatalf("alice assigned contest = %d", code)
	}
	if code := get(aliceCookies, otherID.Hex()); code != http.StatusForbidden {
		t.Fatalf("alice other contest = %d", code)
	}

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	status, titles, _ = list(adminCookies)
	if status != http.StatusOK || len(titles) != 4 {
		t.Fatalf("admin list = %d %+v", status, titles)
	}
	ownerCookies := loginAssetUser(t, server.URL, "owner", "secret")
	if code := get(ownerCookies, ownerID.Hex()); code != http.StatusOK {
		t.Fatalf("owner assigned contest = %d", code)
	}

	// Homework uses the same assignment middleware and list filter.
	homeworkID := bson.NewObjectID()
	doc := contestAssignDoc(homeworkID, "限定作业", 9, []string{"2025"})
	doc["rule"] = "homework"
	if _, err := db.Collection("document").InsertOne(ctx, doc); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/homeworks/"+homeworkID.Hex(), nil)
	for _, cookie := range aliceCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("alice assigned homework = %d", resp.StatusCode)
	}
}

func contestAssignDoc(id bson.ObjectID, title string, owner int64, assign []string) bson.M {
	return bson.M{
		"_id": id, "docId": id, "domainId": "system", "docType": 30,
		"title": title, "rule": "acm", "owner": owner, "assign": assign,
		"beginAt": time.Now().Add(-time.Hour), "endAt": time.Now().Add(time.Hour),
	}
}
