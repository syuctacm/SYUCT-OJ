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
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestProblemStarIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_problem_star_test_%d", time.Now().UnixNano())
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
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756),
	}); err != nil {
		t.Fatal(err)
	}
	first := bson.NewObjectID()
	second := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": first, "domainId": "system", "docType": 10, "docId": int64(1001), "pid": "P1001",
			"title": "A + B", "tag": []string{"math"}, "hidden": false},
		bson.M{"_id": second, "domainId": "system", "docType": 10, "docId": int64(1002), "pid": "P1002",
			"title": "C + D", "tag": []string{"math"}, "hidden": false},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, problemRepo, nil, nil, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	cookies := loginAssetUser(t, server.URL, "alice", "secret")

	post := func(path, body string, withCookies bool) (int, []byte) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if withCookies {
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}
	get := func(path string, withCookies bool) (int, []byte) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if withCookies {
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}

	if status, _ := post("/api/v1/problems/P1001/star", `{"star":true}`, false); status != http.StatusUnauthorized {
		t.Fatalf("anonymous star = %d", status)
	}
	if status, _ := get("/api/v1/starred-problems", false); status != http.StatusUnauthorized {
		t.Fatalf("anonymous starred list = %d", status)
	}

	if status, body := post("/api/v1/problems/P1001/star", `{"star":true}`, true); status != http.StatusOK {
		t.Fatalf("star problem = %d: %s", status, body)
	}
	if status, body := post("/api/v1/problems/P1002/star", `{"star":true}`, true); status != http.StatusOK {
		t.Fatalf("star second problem = %d: %s", status, body)
	}

	status, body := get("/api/v1/problems/P1001", true)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"starred":true`)) {
		t.Fatalf("detail starred = %d: %s", status, body)
	}

	status, body = get("/api/v1/starred-problems", true)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"pid":"P1002"`)) || !bytes.Contains(body, []byte(`"pid":"P1001"`)) {
		t.Fatalf("starred list = %d: %s", status, body)
	}

	if status, _ := post("/api/v1/problems/P1001/star", `{"star":false}`, true); status != http.StatusOK {
		t.Fatalf("unstar = %d", status)
	}
	status, body = get("/api/v1/starred-problems", true)
	if status != http.StatusOK || bytes.Contains(body, []byte(`"pid":"P1001"`)) {
		t.Fatalf("starred list after unstar = %d: %s", status, body)
	}
	status, body = get("/api/v1/problems/P1001", true)
	if status != http.StatusOK || bytes.Contains(body, []byte(`"starred":true`)) {
		t.Fatalf("detail after unstar = %d: %s", status, body)
	}
}
