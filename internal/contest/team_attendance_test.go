package contest_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
)

func TestTeamAttendanceIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_team_attendance_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	tid := bson.NewObjectID()
	now := time.Now().UTC()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": contest.DocumentTypeContest,
		"docId": tid, "title": "Team Contest", "rule": "acm", "allowTeam": true,
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "pids": []int64{1},
		"attend": int64(0), "_code": "invite-me",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("vuser").InsertMany(ctx, []any{
		bson.M{"_id": int64(-1000), "uname": "team:1:alpha", "displayName": "Alpha", "members": []int64{1, 2}},
		bson.M{"_id": int64(-1001), "uname": "team:3:beta", "displayName": "Beta", "members": []int64{2, 3}},
	})
	if err != nil {
		t.Fatal(err)
	}

	repo := contest.NewRepository(db)
	if _, err := repo.AttendTeam(ctx, tid.Hex(), -1000, "Alpha", []int64{1, 2}, "wrong"); !errors.Is(err, contest.ErrInvalidInvitationCode) {
		t.Fatalf("wrong team invitation code: %v", err)
	}
	changed, err := repo.AttendTeam(ctx, tid.Hex(), -1000, "Alpha", []int64{1, 2}, "invite-me")
	if err != nil || !changed {
		t.Fatalf("team attend: changed=%v err=%v", changed, err)
	}
	item, err := repo.Get(ctx, tid.Hex())
	if err != nil || item.Attend != 1 {
		t.Fatalf("contest attendance counter: item=%+v err=%v", item, err)
	}
	for _, uid := range []int64{1, 2} {
		status, statusErr := repo.StatusForUser(ctx, tid.Hex(), uid)
		if statusErr != nil || status == nil || status.UID != -1000 || len(status.Members) != 2 || status.DisplayName != "Alpha" {
			t.Fatalf("user %d team status: %+v err=%v", uid, status, statusErr)
		}
		attended, attendedErr := repo.Attended(ctx, tid.Hex(), uid)
		if attendedErr != nil || !attended {
			t.Fatalf("user %d attended: %v err=%v", uid, attended, attendedErr)
		}
	}
	statuses, err := repo.UserStatuses(ctx, 1, []bson.ObjectID{item.DocID})
	if err != nil || statuses[tid.Hex()].UID != -1000 {
		t.Fatalf("contest list status: %+v err=%v", statuses, err)
	}
	if _, err := repo.AttendTeam(ctx, tid.Hex(), -1001, "Beta", []int64{2, 3}, "invite-me"); !errors.Is(err, contest.ErrContestAlreadyAttended) {
		t.Fatalf("second team should conflict, got %v", err)
	}
	if _, err := repo.AttendContest(ctx, tid.Hex(), 2, "invite-me"); !errors.Is(err, contest.ErrContestAlreadyAttended) {
		t.Fatalf("personal attendance should conflict, got %v", err)
	}
	status, err := repo.EarlyEnd(ctx, tid.Hex(), 1)
	if err != nil || status.UID != -1000 || status.EndAt == nil {
		t.Fatalf("team early end: %+v err=%v", status, err)
	}
}
