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
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
)

func TestContestHackIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_contest_hack_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	config := `type: default
time: 1000ms
memory: 256m
checker_type: testlib
checker: chk.cc
validator: val.cc
subtasks:
  - score: 100
    type: min
    cases:
      - input: 1.in
        output: 1.out
`
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
		"docId": int64(9010), "pid": "PCF", "title": "Codeforces Hack", "owner": int64(1), "config": config,
		"data": []bson.M{
			{"_id": "problem/system/9010/testdata/1.in", "name": "1.in", "size": int64(2)},
			{"_id": "problem/system/9010/testdata/1.out", "name": "1.out", "size": int64(2)},
			{"_id": "problem/system/9010/testdata/chk.cc", "name": "chk.cc", "size": int64(10)},
			{"_id": "problem/system/9010/testdata/val.cc", "name": "val.cc", "size": int64(10)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	contestID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": contest.DocumentTypeContest, "docId": contestID,
		"title": "Codeforces Round", "rule": "codeforces", "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
		"pids": []int64{9010}, "attend": int64(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	problems := problem.NewRepository(db)
	repo := submission.NewRepository(db, problems)
	store := judge.NewFileStore(t.TempDir())
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	repo.SetFileStore(store)
	target, err := repo.Create(ctx, 2, "PCF", "cpp", "int main(){}", contestID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	own, err := repo.Create(ctx, 1, "PCF", "cpp", "int main(){}", contestID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").UpdateMany(ctx, bson.M{"_id": bson.M{"$in": []bson.ObjectID{target.ID, own.ID}}}, bson.M{"$set": bson.M{"status": submission.StatusAccepted}}); err != nil {
		t.Fatal(err)
	}
	hack, err := repo.CreateHack(ctx, 1, "PCF", target.ID.Hex(), []byte("1 2\n"), contestID.Hex())
	if err != nil {
		t.Fatalf("create contest hack: %v", err)
	}
	if hack.Contest == nil || *hack.Contest != contestID || hack.HackTarget == nil || *hack.HackTarget != target.ID {
		t.Fatalf("unexpected contest hack record: %+v", hack)
	}
	if _, err := repo.CreateHack(ctx, 1, "PCF", target.ID.Hex(), []byte("1 2\n")); err == nil {
		t.Fatal("contest target without contest id was accepted")
	}
	if err := repo.ApplyJudgeResult(ctx, map[string]any{"rid": hack.ID.Hex(), "status": submission.StatusHackSuccessful}); err != nil {
		t.Fatal(err)
	}
	count, err := db.Collection("task").CountDocuments(ctx, bson.M{"meta.hackRejudge": "hack-" + hack.ID.Hex() + ".in"})
	if err != nil || count != 2 {
		t.Fatalf("contest hack rejudge tasks = %d err=%v", count, err)
	}
}
