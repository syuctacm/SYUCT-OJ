package contest_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
)

func TestContestRepositoryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_contest_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	oldID := bson.NewObjectID()
	newID := bson.NewObjectID()
	_, err = db.Collection("document").InsertMany(ctx, []any{
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 30,
			"docId": oldID, "title": "Old Contest", "rule": "acm",
			"beginAt": time.Now().Add(-48 * time.Hour), "endAt": time.Now().Add(-47 * time.Hour),
			"attend": int64(10), "pids": []int64{1, 2},
		},
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 30,
			"docId": newID, "title": "New Contest", "rule": "oi",
			"beginAt": time.Now().Add(-2 * time.Hour), "endAt": time.Now().Add(-1 * time.Hour),
			"attend": int64(20), "pids": []int64{2, 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	homeworkID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 30,
		"docId": homeworkID, "title": "Homework 1", "rule": "homework",
		"beginAt": time.Now().Add(-time.Hour), "endAt": time.Now().Add(time.Hour),
		"penaltySince": time.Now().Add(30 * time.Minute), "pids": []int64{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "First"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2), "pid": "P2", "title": "Second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := contest.NewRepository(db)
	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 || items[0].Title != "New Contest" {
		t.Fatalf("unexpected contest list: %+v", items)
	}
	items, err = repo.ListFiltered(ctx, "New", "")
	if err != nil || len(items) != 1 || items[0].Title != "New Contest" {
		t.Fatalf("contest search: items=%+v err=%v", items, err)
	}
	items, err = repo.ListFiltered(ctx, "", "acm")
	if err != nil || len(items) != 1 || items[0].Title != "Old Contest" {
		t.Fatalf("contest rule filter: items=%+v err=%v", items, err)
	}
	found, err := repo.Get(ctx, newID.Hex())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Title != "New Contest" || len(found.PIDs) != 2 {
		t.Fatalf("unexpected contest: %+v", found)
	}
	_, problems, err := repo.Problems(ctx, newID.Hex())
	if err != nil {
		t.Fatalf("contest problems: %v", err)
	}
	if len(problems) != 2 || problems[0].DocID != 2 || problems[1].DocID != 1 {
		t.Fatalf("unexpected contest problem order: %+v", problems)
	}
	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(7), "pid": int64(2), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(7), "pid": int64(1), "status": 2},
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(7), "pid": int64(1), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(8), "pid": int64(2), "status": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, rows, err := repo.Scoreboard(ctx, newID.Hex())
	if err != nil {
		t.Fatalf("scoreboard: %v", err)
	}
	if len(rows) != 2 || rows[0].UID != 7 || rows[0].Solved != 2 || rows[1].UID != 8 {
		t.Fatalf("unexpected scoreboard: %+v", rows)
	}
	homeworks, err := repo.ListHomework(ctx)
	if err != nil {
		t.Fatalf("list homework: %v", err)
	}
	if len(homeworks) != 1 || homeworks[0].Title != "Homework 1" || homeworks[0].PenaltySince == nil {
		t.Fatalf("unexpected homeworks: %+v", homeworks)
	}
	homeworks, err = repo.ListHomeworkFiltered(ctx, "Homework")
	if err != nil || len(homeworks) != 1 {
		t.Fatalf("homework search: homeworks=%+v err=%v", homeworks, err)
	}
	if _, err := repo.GetHomework(ctx, oldID.Hex()); err != contest.ErrNotFound {
		t.Fatalf("non-homework rule should not be returned: %v", err)
	}
	if _, err := repo.Get(ctx, "invalid"); err != contest.ErrNotFound {
		t.Fatalf("invalid id error = %v", err)
	}
}
