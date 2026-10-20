package problem_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

func TestProblemRepositoryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_problem_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	_, err = db.Collection("document").InsertMany(ctx, []any{
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
			"docId": int64(1), "pid": "P1001", "title": "A+B", "content": "1+1",
			"tag": []string{"math"}, "difficulty": int32(1),
			"nSubmit": int64(10), "nAccept": int64(5), "sort": "P1001",
		},
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
			"docId": int64(2), "pid": "P1002", "title": "Graph", "content": "graph",
			"tag": []string{"graph"}, "difficulty": int32(3),
			"nSubmit": int64(4), "nAccept": int64(2), "sort": "P1002",
		},
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
			"docId": int64(3), "pid": "P1003", "title": "Hidden", "hidden": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := problem.NewRepository(db)

	result, err := repo.List(ctx, problem.ListOptions{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("visible list total=%d items=%d", result.Total, len(result.Items))
	}

	result, err = repo.List(ctx, problem.ListOptions{Page: 1, Limit: 20, Query: "graph"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result.Total != 1 || result.Items[0].PID != "P1002" {
		t.Fatalf("search result: %+v", result)
	}

	difficulty := 1
	result, err = repo.List(ctx, problem.ListOptions{Page: 1, Limit: 20, Difficulty: &difficulty})
	if err != nil {
		t.Fatalf("difficulty filter: %v", err)
	}
	if result.Total != 1 || result.Items[0].PID != "P1001" {
		t.Fatalf("difficulty result: %+v", result)
	}

	pdoc, err := repo.Get(ctx, "P1002")
	if err != nil {
		t.Fatalf("get by pid: %v", err)
	}
	if pdoc.Content != "graph" {
		t.Fatalf("content = %q", pdoc.Content)
	}
	if _, err := repo.Get(ctx, "2"); err != nil {
		t.Fatalf("get by docId: %v", err)
	}
	if _, err := repo.Get(ctx, "P1003"); err != problem.ErrNotFound {
		t.Fatalf("hidden problem should not be visible: %v", err)
	}
}
