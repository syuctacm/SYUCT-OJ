package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestConfiguredSubmissionRateLimit(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_submission_rate_test_%d", time.Now().UnixNano())
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
		"_id": int64(7001), "uname": "ratelimit", "unameLower": "ratelimit",
		"mail": "ratelimit@example.com", "mailLower": "ratelimit@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
		"docId": int64(7001), "pid": "PRATE", "title": "Rate Limit", "hidden": false,
		"config": "type: default\ntime: 1000ms\nmemory: 256m\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("system").InsertMany(ctx, []any{
		bson.M{"_id": "limit.by_user", "value": true},
		bson.M{"_id": "limit.submission_user", "value": 1},
		bson.M{"_id": "limit.submission", "value": 60},
		bson.M{"_id": "limit.codelength", "value": 1024},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	submissionRepo := submission.NewRepository(db, problemRepo)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, nil, userRepo)
	router := api.NewRouter(
		config.Config{}, db, nil, authHandler, problemRepo, submissionHandler, nil, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	cookies := loginAssetUser(t, server.URL, "ratelimit", "secret")
	body := map[string]string{"pid": "PRATE", "lang": "cpp", "code": "int main(){}"}
	status, response := postJSON(t, server.URL+"/api/v1/submissions", body, cookies)
	if status != http.StatusAccepted {
		t.Fatalf("first submission = %d: %s", status, response)
	}
	status, response = postJSON(t, server.URL+"/api/v1/submissions", body, cookies)
	if status != http.StatusTooManyRequests {
		t.Fatalf("rate-limited submission = %d: %s", status, response)
	}
}
