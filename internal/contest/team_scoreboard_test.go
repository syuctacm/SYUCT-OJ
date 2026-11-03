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

func TestTeamScoreboardIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_team_scoreboard_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	now := time.Now().UTC()
	tid := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": tid, "domainId": "system", "docType": contest.DocumentTypeContest, "docId": tid,
		"title": "Team Scoreboard", "rule": "acm", "beginAt": now.Add(-2 * time.Hour),
		"endAt": now.Add(time.Hour), "pids": []int64{1, 2}, "attend": int64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("document.status").InsertOne(ctx, bson.M{
		"domainId": "system", "docType": contest.DocumentTypeContest, "docId": tid,
		"uid": int64(-1000), "attend": 1, "displayName": "Alpha", "members": []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-90 * time.Minute)), "contest": tid, "domainId": "system", "uid": int64(-1000), "pid": int64(1), "status": 2, "score": 0},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-80 * time.Minute)), "contest": tid, "domainId": "system", "uid": int64(-1000), "pid": int64(1), "status": 1, "score": 100},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(now.Add(-70 * time.Minute)), "contest": tid, "domainId": "system", "uid": int64(-1000), "pid": int64(2), "status": 1, "score": 100},
	})
	if err != nil {
		t.Fatal(err)
	}

	repo := contest.NewRepository(db)
	item, rows, err := repo.Scoreboard(ctx, tid.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "Team Scoreboard" || len(rows) != 1 {
		t.Fatalf("unexpected team scoreboard: item=%+v rows=%+v", item, rows)
	}
	row := rows[0]
	if row.UID != -1000 || row.DisplayName != "Alpha" || len(row.Members) != 2 || row.Solved != 2 || row.Penalty <= 0 {
		t.Fatalf("unexpected team row: %+v", row)
	}
	statuses, err := repo.Statuses(ctx, tid.Hex())
	if err != nil || len(statuses) != 1 || statuses[0].DisplayName != "Alpha" || len(statuses[0].Members) != 2 {
		t.Fatalf("unexpected team statuses: %+v err=%v", statuses, err)
	}
	if err := repo.SetUnrank(ctx, tid.Hex(), -1000, true); err != nil {
		t.Fatalf("set team unrank: %v", err)
	}
	if _, rows, err = repo.Scoreboard(ctx, tid.Hex()); err != nil || len(rows) != 0 {
		t.Fatalf("unranked team row: rows=%+v err=%v", rows, err)
	}
}
