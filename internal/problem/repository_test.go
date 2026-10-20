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
			"config": "type: default\ntime: 1000ms\nmemory: 256m\n",
		},
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
			"docId": int64(3), "pid": "P1003", "title": "Hidden", "hidden": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	acceptedRID := bson.NewObjectID()
	wrongRID := bson.NewObjectID()
	_, err = db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(7), "uname": "alice", "displayName": "Alice"},
		bson.M{"_id": int64(8), "uname": "bob", "displayName": "Bob"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": acceptedRID, "uid": int64(7), "pid": int64(1), "domainId": "system", "status": 1},
		bson.M{"_id": wrongRID, "uid": int64(7), "pid": int64(2), "domainId": "system", "status": 2},
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

	result, err = repo.List(ctx, problem.ListOptions{Page: 1, Limit: 20, UID: 7})
	if err != nil {
		t.Fatalf("user status: %v", err)
	}
	if result.Items[0].UserStatus == nil || result.Items[0].UserStatus.RID != acceptedRID.Hex() || result.Items[0].UserStatus.Status != 1 {
		t.Fatalf("accepted status: %+v", result.Items[0])
	}
	if result.Items[1].UserStatus == nil || result.Items[1].UserStatus.RID != wrongRID.Hex() || result.Items[1].UserStatus.Status != 2 {
		t.Fatalf("wrong answer status: %+v", result.Items[1])
	}

	result, err = repo.List(ctx, problem.ListOptions{Page: 1, Limit: 20, Sort: "recent"})
	if err != nil {
		t.Fatalf("recent sort: %v", err)
	}
	if result.Items[0].PID != "P1002" {
		t.Fatalf("recent sort first = %s", result.Items[0].PID)
	}

	pdoc, err := repo.Get(ctx, "P1002")
	if err != nil {
		t.Fatalf("get by pid: %v", err)
	}
	if pdoc.Content != "graph" {
		t.Fatalf("content = %q", pdoc.Content)
	}
	if pdoc.ProblemType != "default" || pdoc.TimeLimit != "1000ms" || pdoc.MemoryLimit != "256m" {
		t.Fatalf("config summary: %+v", pdoc)
	}
	if _, err := repo.Get(ctx, "2"); err != nil {
		t.Fatalf("get by docId: %v", err)
	}
	if _, err := repo.Get(ctx, "P1003"); err != problem.ErrNotFound {
		t.Fatalf("hidden problem should not be visible: %v", err)
	}
	if _, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(1), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(8), "pid": int64(1), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(8), "pid": int64(1), "domainId": "system", "status": 2},
	}); err != nil {
		t.Fatal(err)
	}
	tags, err := repo.TagStats(ctx)
	if err != nil || len(tags) != 2 || tags[0].Tag != "graph" || tags[0].Count != 1 {
		t.Fatalf("tag stats: tags=%+v err=%v", tags, err)
	}
	ranking, err := repo.TagRanking(ctx, "math", 1, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if ranking.ProblemCount != 1 || ranking.TotalUsers != 2 || len(ranking.Items) != 2 {
		t.Fatalf("tag ranking: %+v", ranking)
	}
	if ranking.Items[0].UID != 7 || ranking.Items[0].Username != "alice" {
		t.Fatalf("tag ranking first: %+v", ranking.Items[0])
	}
	ranking, err = repo.TagRanking(ctx, "math", 1, 20, "bob")
	if err != nil || ranking.MatchedCount != 1 || ranking.Items[0].UID != 8 {
		t.Fatalf("tag ranking search: ranking=%+v err=%v", ranking, err)
	}
}
