package submission_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
)

func TestTeamSubmissionIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_team_submission_test_%d", time.Now().UnixNano())
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
		"docId": int64(1), "pid": "P1001", "title": "A+B", "owner": int64(1), "config": config,
		"data": []bson.M{
			{"_id": "problem/system/1/1.in", "name": "1.in", "size": int64(2)},
			{"_id": "problem/system/1/1.out", "name": "1.out", "size": int64(2)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	contestID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID,
		"title": "Team Contest", "rule": "acm", "pids": []int64{1}, "attend": int64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("vuser").InsertOne(ctx, bson.M{
		"_id": int64(-1000), "uname": "team:1:alpha", "displayName": "Alpha", "members": []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	repo := submission.NewRepository(db, problemRepo)
	var acceptedUID int64
	repo.SetAcceptedHook(func(_ context.Context, _ bson.ObjectID, uid, _ int64) {
		acceptedUID = uid
		_ = contestRepo.AddBalloon(ctx, contestID, uid, 1)
	})
	record, err := repo.Create(ctx, -1000, "P1001", "cpp", "int main(){}", contestID.Hex())
	if err != nil {
		t.Fatalf("create team submission: %v", err)
	}
	if record.UID != -1000 || record.Contest == nil || *record.Contest != contestID {
		t.Fatalf("unexpected team record: %+v", record)
	}
	var task struct {
		UID int64 `bson:"uid"`
	}
	if err := db.Collection("task").FindOne(ctx, bson.M{"rid": record.ID}).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if task.UID != -1000 {
		t.Fatalf("judge task uid = %d", task.UID)
	}
	list, err := repo.List(ctx, submission.ListFilter{Page: 1, Limit: 10, UID: -1000})
	if err != nil {
		t.Fatalf("list team submissions: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].Submitter == nil ||
		list.Items[0].Submitter.ID != -1000 || list.Items[0].Submitter.DisplayName != "Alpha" {
		t.Fatalf("team record enrichment: %+v", list)
	}
	if err := repo.ApplyJudgeResult(ctx, map[string]any{
		"key": "end", "rid": record.ID.Hex(), "status": float64(1), "score": float64(100),
	}); err != nil {
		t.Fatalf("apply team judge result: %v", err)
	}
	if acceptedUID != -1000 {
		t.Fatalf("accepted hook uid = %d", acceptedUID)
	}
	var balloon struct {
		UID int64 `bson:"uid"`
	}
	if err := db.Collection("contest.balloon").FindOne(ctx, bson.M{"docId": contestID, "pid": int64(1)}).Decode(&balloon); err != nil {
		t.Fatalf("team balloon: %v", err)
	}
	if balloon.UID != -1000 {
		t.Fatalf("team balloon uid = %d", balloon.UID)
	}
}
