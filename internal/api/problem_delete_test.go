package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestProblemDeleteIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_problem_delete_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
			"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(2), "uname": "owner", "unameLower": "owner", "mail": "owner@example.com",
			"mailLower": "owner@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(3), "uname": "other", "unameLower": "other", "mail": "other@example.com",
			"mailLower": "other@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	discussionID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1),
			"pid": "P1001", "title": "Delete me", "owner": int64(2), "nSubmit": int64(1), "nAccept": int64(1),
			"config": "type: default\ntime: 1000ms\nmemory: 256m\n", "data": []any{}, "additional_file": []any{}},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 11, "parentType": 10, "parentId": int64(1),
			"owner": int64(2), "content": "solution"},
		bson.M{"_id": discussionID, "domainId": "system", "docType": 21, "parentType": 10, "parentId": int64(1),
			"title": "discussion", "owner": int64(2)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 22, "parentType": 21, "parentId": discussionID,
			"owner": int64(2), "content": "reply"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2),
			"pid": "P1002", "title": "Admin deletes", "owner": int64(1), "config": "type: default\n", "data": []any{}, "additional_file": []any{}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "pid": int64(1), "uid": int64(2), "lang": "cpp", "code": "x", "status": 1},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "pid": int64(2), "uid": int64(2), "lang": "cpp", "code": "y", "status": 1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("task").InsertOne(ctx, bson.M{"domainId": "system", "pid": int64(1), "type": "judge"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document.status").InsertOne(ctx, bson.M{
		"domainId": "system", "docType": 10, "docId": int64(1), "uid": int64(3), "star": true,
	}); err != nil {
		t.Fatal(err)
	}

	storeRoot := t.TempDir()
	store := judge.NewFileStore(storeRoot)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	problemDir := filepath.Join(storeRoot, "problem", "system", "1", "testdata")
	if err := os.MkdirAll(problemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(problemDir, "1.in"), []byte("1 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	judgeHandler := judge.NewHandler("judge", "secret", sessions, store, judge.NewHub(nil))
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		judgeHandler, authHandler, problemRepo, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	owner := loginAssetUser(t, server.URL, "owner", "secret")
	other := loginAssetUser(t, server.URL, "other", "secret")
	admin := loginAssetUser(t, server.URL, "admin", "secret")
	call := func(pid string, cookies []*http.Cookie) int {
		req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/problems/"+pid, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code := call("P1001", nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete = %d", code)
	}
	if code := call("P1001", other); code != http.StatusForbidden {
		t.Fatalf("non-owner delete = %d", code)
	}
	if code := call("P1001", owner); code != http.StatusNoContent {
		t.Fatalf("owner delete = %d", code)
	}

	count := func(collection string, filter bson.M) int64 {
		n, err := db.Collection(collection).CountDocuments(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count("document", bson.M{"docType": 10, "docId": int64(1)}); n != 0 {
		t.Fatalf("problem document remains: %d", n)
	}
	if n := count("document", bson.M{"docType": 11, "parentId": int64(1)}); n != 0 {
		t.Fatalf("solution remains: %d", n)
	}
	if n := count("document", bson.M{"docType": 21, "parentId": int64(1)}); n != 0 {
		t.Fatalf("discussion remains: %d", n)
	}
	if n := count("document", bson.M{"docType": 22, "parentId": discussionID}); n != 0 {
		t.Fatalf("discussion reply remains: %d", n)
	}
	if n := count("record", bson.M{"pid": int64(1)}); n != 0 {
		t.Fatalf("record remains: %d", n)
	}
	if n := count("task", bson.M{"pid": int64(1)}); n != 0 {
		t.Fatalf("task remains: %d", n)
	}
	if n := count("document.status", bson.M{"docType": 10, "docId": int64(1)}); n != 0 {
		t.Fatalf("status remains: %d", n)
	}
	if _, err := os.Stat(filepath.Join(storeRoot, "problem", "system", "1")); !os.IsNotExist(err) {
		t.Fatalf("problem files remain: %v", err)
	}
	// The unrelated problem is untouched.
	if n := count("document", bson.M{"docType": 10, "docId": int64(2)}); n != 1 {
		t.Fatalf("unrelated problem removed")
	}
	if code := call("P1002", admin); code != http.StatusNoContent {
		t.Fatalf("admin delete = %d", code)
	}
}
