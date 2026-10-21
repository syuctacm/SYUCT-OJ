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
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "First", "config": "type: default\ntime: 1000\nmemory: 256"},
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
	if p := problems[1]; p.ProblemType != "default" || p.TimeMin != "1000" || p.TimeMax != "1000" || p.MemoryMin != "256" || p.MemoryMax != "256" {
		t.Fatalf("unexpected config summary: %+v", p)
	}
	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(7), "pid": int64(2), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(7), "pid": int64(1), "status": 2},
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(7), "pid": int64(1), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(8), "pid": int64(2), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "contest": newID, "domainId": "system", "uid": int64(10), "pid": int64(1), "status": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Collection("document.status").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "docType": 30, "docId": newID, "uid": int64(7), "attend": 1},
		bson.M{"domainId": "system", "docType": 30, "docId": newID, "uid": int64(8), "attend": 1},
		bson.M{"domainId": "system", "docType": 30, "docId": newID, "uid": int64(9), "attend": 1},
	}); err != nil {
		t.Fatal(err)
	}
	_, rows, err := repo.Scoreboard(ctx, newID.Hex())
	if err != nil {
		t.Fatalf("scoreboard: %v", err)
	}
	if len(rows) != 4 || rows[0].UID != 7 || rows[0].Solved != 2 || rows[1].UID != 8 {
		t.Fatalf("unexpected scoreboard: %+v", rows)
	}
	byUID := map[int64]contest.ScoreboardRow{}
	for _, row := range rows {
		byUID[row.UID] = row
	}
	if row, ok := byUID[9]; !ok || row.Solved != 0 || len(row.Problems) != 0 {
		t.Fatalf("expected attendee without submissions to be listed: %+v", row)
	}
	if cell := byUID[7].Problems["2"]; !cell.Solved || cell.RID == "" || cell.Attempts != 0 {
		t.Fatalf("expected accepted record id on solved cell: %+v", cell)
	}
	if cell := byUID[7].Problems["1"]; !cell.Solved || cell.RID == "" || cell.Attempts != 1 {
		t.Fatalf("expected attempts before accept: %+v", cell)
	}
	if cell := byUID[8].Problems["1"]; cell.RID != "" || cell.Attempts != 0 {
		t.Fatalf("unexpected empty cell: %+v", cell)
	}
	if cell := byUID[10].Problems["1"]; cell.Solved || cell.RID == "" || cell.Attempts != 1 {
		t.Fatalf("expected record id on failed attempt: %+v", cell)
	}
	statuses, err := repo.UserProblemStatus(ctx, 7, []int64{1, 2})
	if err != nil || statuses[1].Status != 1 || statuses[1].RID == "" || statuses[2].RID == "" {
		t.Fatalf("unexpected user problem status: %+v err=%v", statuses, err)
	}
	if statuses, err = repo.UserProblemStatus(ctx, 10, []int64{1, 2}); err != nil || statuses[1].Status != 2 || statuses[1].RID == "" || statuses[2].RID != "" {
		t.Fatalf("unexpected failed attempt status: %+v err=%v", statuses, err)
	}
	if statuses, err = repo.UserProblemStatus(ctx, 0, []int64{1}); err != nil || len(statuses) != 0 {
		t.Fatalf("anonymous problem status: %+v err=%v", statuses, err)
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

	created, err := repo.AddClarification(ctx, newID.Hex(), 7, 1, "题目一的时间限制是多少？")
	if err != nil || created.Subject != 1 || created.Content == "" || created.Owner != 7 {
		t.Fatalf("add clarification: %+v err=%v", created, err)
	}
	own, err := repo.ListClarifications(ctx, newID.Hex(), 7, false)
	if err != nil || len(own) != 1 || own[0].ID != created.ID {
		t.Fatalf("list own clarifications: %+v err=%v", own, err)
	}
	if others, err := repo.ListClarifications(ctx, newID.Hex(), 8, false); err != nil || len(others) != 0 {
		t.Fatalf("clarifications should be private: %+v err=%v", others, err)
	}
	jury, err := repo.AddClarification(ctx, newID.Hex(), 0, 0, "请注意比赛纪律。")
	if err != nil {
		t.Fatal(err)
	}
	if others, err := repo.ListClarifications(ctx, newID.Hex(), 8, false); err != nil || len(others) != 1 || others[0].ID != jury.ID {
		t.Fatalf("jury announcements should be visible: %+v err=%v", others, err)
	}
	all, err := repo.ListClarifications(ctx, newID.Hex(), 1, true)
	if err != nil || len(all) != 2 {
		t.Fatalf("list all clarifications: %+v err=%v", all, err)
	}
	replied, err := repo.AddClarificationReply(ctx, created.ID.Hex(), 1, "1000ms。")
	if err != nil || len(replied.Reply) != 1 || replied.Reply[0].Content != "1000ms。" {
		t.Fatalf("reply clarification: %+v err=%v", replied, err)
	}
	if _, err := repo.AddClarification(ctx, newID.Hex(), 7, 999, "bad"); err != contest.ErrInvalidClarificationSubject {
		t.Fatalf("invalid subject error = %v", err)
	}
	if _, err := repo.AddClarification(ctx, newID.Hex(), 7, 1, "   "); err != contest.ErrEmptyClarificationContent {
		t.Fatalf("empty content error = %v", err)
	}
	if _, err := repo.AddClarificationReply(ctx, "invalid", 1, "x"); err != contest.ErrClarificationNotFound {
		t.Fatalf("invalid clarification id error = %v", err)
	}
	if _, err := repo.AddClarification(ctx, "invalid", 7, 0, "x"); err != contest.ErrNotFound {
		t.Fatalf("invalid contest id error = %v", err)
	}

	attended, err := repo.Attended(ctx, newID.Hex(), 10)
	if err != nil || attended {
		t.Fatalf("unexpected initial attendance: attended=%v err=%v", attended, err)
	}
	changed, err := repo.Attend(ctx, newID.Hex(), 10)
	if err != nil || !changed {
		t.Fatalf("attend: changed=%v err=%v", changed, err)
	}
	changed, err = repo.Attend(ctx, newID.Hex(), 10)
	if err != nil || changed {
		t.Fatalf("repeat attend should be idempotent: changed=%v err=%v", changed, err)
	}
	attended, err = repo.Attended(ctx, newID.Hex(), 10)
	if err != nil || !attended {
		t.Fatalf("attendance after attend: attended=%v err=%v", attended, err)
	}
	if attended, err = repo.Attended(ctx, newID.Hex(), 0); err != nil || attended {
		t.Fatalf("anonymous attendance: attended=%v err=%v", attended, err)
	}
	updated, err := repo.Get(ctx, newID.Hex())
	if err != nil || updated.Attend != 21 {
		t.Fatalf("attend counter: attend=%d err=%v", updated.Attend, err)
	}
	if _, err := repo.Attend(ctx, "invalid", 7); err != contest.ErrNotFound {
		t.Fatalf("attend invalid id error = %v", err)
	}
}
