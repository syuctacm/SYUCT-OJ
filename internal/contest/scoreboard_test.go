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

func TestScoreboardRulesIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_scoreboard_rules_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	now := time.Now().Truncate(time.Second)
	acmID := bson.NewObjectID()
	oiID := bson.NewObjectID()
	ioiID := bson.NewObjectID()
	strictID := bson.NewObjectID()
	homeworkID := bson.NewObjectID()
	ledoID := bson.NewObjectID()
	contestDocs := []any{
		bson.M{"_id": acmID, "docId": acmID, "domainId": "system", "docType": 30, "title": "ACM", "rule": "acm", "owner": int64(1), "beginAt": now.Add(-2 * time.Hour), "endAt": now.Add(time.Hour), "lockAt": now.Add(-30 * time.Minute), "pids": []int64{1, 2}},
		bson.M{"_id": oiID, "docId": oiID, "domainId": "system", "docType": 30, "title": "OI", "rule": "oi", "owner": int64(1), "beginAt": now.Add(-3 * time.Hour), "endAt": now.Add(-time.Hour), "pids": []int64{1, 2}},
		bson.M{"_id": ioiID, "docId": ioiID, "domainId": "system", "docType": 30, "title": "IOI", "rule": "ioi", "owner": int64(1), "beginAt": now.Add(-3 * time.Hour), "endAt": now.Add(-time.Hour), "pids": []int64{1}},
		bson.M{"_id": strictID, "docId": strictID, "domainId": "system", "docType": 30, "title": "Strict IOI", "rule": "strictioi", "owner": int64(1), "beginAt": now.Add(-3 * time.Hour), "endAt": now.Add(-time.Hour), "pids": []int64{1}},
		bson.M{"_id": homeworkID, "docId": homeworkID, "domainId": "system", "docType": 30, "title": "Homework", "rule": "homework", "owner": int64(1), "beginAt": now.Add(-3 * time.Hour), "endAt": now.Add(time.Hour), "penaltySince": now.Add(-2 * time.Hour), "penaltyRules": bson.M{"1": 0.8}, "pids": []int64{1}},
		bson.M{"_id": ledoID, "docId": ledoID, "domainId": "system", "docType": 30, "title": "Ledo", "rule": "ledo", "owner": int64(1), "beginAt": now.Add(-3 * time.Hour), "endAt": now.Add(time.Hour), "pids": []int64{1}},
	}
	if _, err := db.Collection("document").InsertMany(ctx, contestDocs); err != nil {
		t.Fatal(err)
	}
	statusDocs := []any{}
	for _, tid := range []bson.ObjectID{acmID, oiID, ioiID, strictID, homeworkID, ledoID} {
		for _, uid := range []int64{1, 2, 3} {
			statusDocs = append(statusDocs, bson.M{"domainId": "system", "docType": 30, "docId": tid, "uid": uid, "attend": 1})
		}
	}
	if _, err := db.Collection("document.status").InsertMany(ctx, statusDocs); err != nil {
		t.Fatal(err)
	}

	records := []any{
		// ACM: user 1 has one WA plus AC; user 2 ACs later. Post-lock AC is pending.
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-90 * time.Minute)), "contest": acmID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 2, "score": 0},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-80 * time.Minute)), "contest": acmID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 1, "score": 100},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-70 * time.Minute)), "contest": acmID, "domainId": "system", "uid": int64(2), "pid": int64(1), "status": 1, "score": 100},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-10 * time.Minute)), "contest": acmID, "domainId": "system", "uid": int64(1), "pid": int64(2), "status": 1, "score": 100},

		// OI: best score per problem.
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-150 * time.Minute)), "contest": oiID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 2, "score": 50},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-140 * time.Minute)), "contest": oiID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 1, "score": 90},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-130 * time.Minute)), "contest": oiID, "domainId": "system", "uid": int64(1), "pid": int64(2), "status": 2, "score": 80},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-120 * time.Minute)), "contest": oiID, "domainId": "system", "uid": int64(2), "pid": int64(1), "status": 1, "score": 100},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-110 * time.Minute)), "contest": ioiID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 1, "score": 75},

		// Strict IOI merges the best score of each subtask.
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-150 * time.Minute)), "contest": strictID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 2, "score": 90, "subtasks": bson.M{"0": bson.M{"status": 2, "score": 50}, "1": bson.M{"status": 2, "score": 30}}},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-140 * time.Minute)), "contest": strictID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 1, "score": 100, "subtasks": bson.M{"0": bson.M{"status": 1, "score": 60}, "1": bson.M{"status": 1, "score": 40}}},

		// Homework uses the latest submission and applies the delay coefficient.
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-150 * time.Minute)), "contest": homeworkID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 1, "score": 100},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-30 * time.Minute)), "contest": homeworkID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 2, "score": 80},

		// Ledo keeps the best decayed score while counting valid attempts.
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-150 * time.Minute)), "contest": ledoID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 2, "score": 100},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-140 * time.Minute)), "contest": ledoID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 2, "score": 100},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-130 * time.Minute)), "contest": ledoID, "domainId": "system", "uid": int64(1), "pid": int64(1), "status": 1, "score": 100},
	}
	if _, err := db.Collection("record").InsertMany(ctx, records); err != nil {
		t.Fatal(err)
	}
	repo := contest.NewRepository(db)
	byUID := func(rows []contest.ScoreboardRow) map[int64]contest.ScoreboardRow {
		result := make(map[int64]contest.ScoreboardRow, len(rows))
		for _, row := range rows {
			result[row.UID] = row
		}
		return result
	}

	_, rows, err := repo.Scoreboard(ctx, acmID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	acm := byUID(rows)
	if acm[1].Solved != 1 || acm[1].Penalty != 60 || acm[1].Problems["2"].Pending != 1 {
		t.Fatalf("unexpected ACM row: %+v", acm[1])
	}
	if !acm[1].Problems["1"].First || acm[2].Problems["1"].First {
		t.Fatalf("unexpected ACM first accept: user1=%+v user2=%+v", acm[1].Problems["1"], acm[2].Problems["1"])
	}

	_, rows, err = repo.Scoreboard(ctx, oiID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	oi := byUID(rows)
	if oi[1].Score != 170 || oi[2].Score != 100 || rows[0].UID != 1 {
		t.Fatalf("unexpected OI rows: %+v", rows)
	}
	if oi[1].Problems["1"].Score != 90 || oi[1].Problems["2"].Score != 80 {
		t.Fatalf("unexpected OI cells: %+v", oi[1].Problems)
	}

	_, rows, err = repo.Scoreboard(ctx, ioiID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if byUID(rows)[1].Score != 75 {
		t.Fatalf("unexpected IOI rows: %+v", rows)
	}

	_, rows, err = repo.Scoreboard(ctx, strictID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	strictCell := byUID(rows)[1].Problems["1"]
	if byUID(rows)[1].Score != 100 || strictCell.Score != 100 || !strictCell.Solved {
		t.Fatalf("unexpected strict IOI row: %+v", byUID(rows)[1])
	}

	_, rows, err = repo.Scoreboard(ctx, homeworkID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	homeworkCell := byUID(rows)[1].Problems["1"]
	if byUID(rows)[1].Score != 64 || homeworkCell.PenaltyScore != 64 || homeworkCell.Score != 80 {
		t.Fatalf("unexpected homework row: %+v", byUID(rows)[1])
	}

	_, rows, err = repo.Scoreboard(ctx, ledoID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	ledoCell := byUID(rows)[1].Problems["1"]
	if byUID(rows)[1].Score != 100 || ledoCell.PenaltyScore != 100 || ledoCell.Attempts != 2 {
		t.Fatalf("unexpected Ledo row: %+v", byUID(rows)[1])
	}
}
