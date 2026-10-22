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
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestRandomProblemIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_random_problem_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1),
			"pid": "P1", "title": "Math", "tag": []string{"math"}, "hidden": false},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2),
			"pid": "P2", "title": "Graph", "tag": []string{"graph"}, "hidden": false},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(3),
			"pid": "P3", "title": "Hidden math", "tag": []string{"math"}, "hidden": true},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, problemRepo, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	call := func(query string) (int, []byte) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/problems/random"+query, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data := make([]byte, 256)
		n, _ := resp.Body.Read(data)
		return resp.StatusCode, data[:n]
	}

	for attempt := 0; attempt < 5; attempt++ {
		status, body := call("")
		if status != http.StatusOK {
			t.Fatalf("random = %d: %s", status, body)
		}
		if !bytes.Contains(body, []byte(`"pid":"P1"`)) && !bytes.Contains(body, []byte(`"pid":"P2"`)) {
			t.Fatalf("random returned hidden or unknown problem: %s", body)
		}
	}
	status, body := call("?q=category:math")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"pid":"P1"`)) {
		t.Fatalf("tagged random = %d: %s", status, body)
	}
	if status, _ := call("?q=category:missing"); status != http.StatusNotFound {
		t.Fatalf("missing tag = %d", status)
	}
}
