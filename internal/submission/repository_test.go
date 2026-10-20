package submission_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
)

func TestSubmissionFlowIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_submission_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	config := "type: default\ntime: 1000ms\nmemory: 256m\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n"
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
		"docId": int64(1), "pid": "P1001", "title": "A+B", "owner": int64(1),
		"config": config,
		"data": []bson.M{
			{"_id": "problem/system/1/1.in", "name": "1.in", "size": int64(2)},
			{"_id": "problem/system/1/1.out", "name": "1.out", "size": int64(2)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	problemRepo := problem.NewRepository(db)
	repo := submission.NewRepository(db, problemRepo)
	record, err := repo.Create(ctx, 7, "P1001", "cpp", "int main(){}")
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	if record.Status != submission.StatusWaiting || record.PID != 1 {
		t.Fatalf("unexpected record: %+v", record)
	}
	taskCount, err := db.Collection("task").CountDocuments(ctx, bson.M{"rid": record.ID})
	if err != nil {
		t.Fatal(err)
	}
	if taskCount != 1 {
		t.Fatalf("task count = %d", taskCount)
	}
	var task bson.M
	if err := db.Collection("task").FindOne(ctx, bson.M{"rid": record.ID}).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if task["type"] != "judge" || task["domainId"] != "system" || task["config"] == nil {
		t.Fatalf("unexpected task: %+v", task)
	}

	if err := repo.ApplyJudgeResult(ctx, map[string]any{
		"key": "end", "rid": record.ID.Hex(), "status": float64(1),
		"score": float64(100), "time": float64(12), "memory": float64(1024),
		"message": "Accepted",
	}); err != nil {
		t.Fatalf("apply result: %v", err)
	}
	updated, err := repo.Get(ctx, record.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != submission.StatusAccepted || updated.Score != 100 {
		t.Fatalf("updated record: %+v", updated)
	}
	if updated.JudgeAt == nil {
		t.Fatal("judgeAt was not set")
	}
}
