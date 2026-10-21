package user_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestRepositoryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_user_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	_, err = db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "Alice", "unameLower": "alice",
		"mail": "Alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1), "bio": "ACMer",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := user.NewRepository(db)
	found, err := repo.FindByLogin(ctx, "ALICE")
	if err != nil {
		t.Fatalf("find by login: %v", err)
	}
	if found.ID != 7 || found.Uname != "Alice" {
		t.Fatalf("unexpected user: %+v", found)
	}
	found, err = repo.FindByID(ctx, 7)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if user.VerifyPassword(found, "secret") != nil {
		t.Fatal("stored password cannot be verified")
	}
	if found.Bio != "ACMer" {
		t.Fatalf("bio = %q", found.Bio)
	}
	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(1), "domainId": "system", "status": 2},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(1), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(2), "domainId": "system", "status": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	submit, accepted, err := repo.Statistics(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if submit != 3 || accepted != 2 {
		t.Fatalf("statistics submit=%d accepted=%d", submit, accepted)
	}
	activityRecords := []any{
		bson.M{"_id": bson.NewObjectIDFromTimestamp(time.Now().Add(-72 * time.Hour)), "uid": int64(9), "pid": int64(10), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(time.Now().Add(-48 * time.Hour)), "uid": int64(9), "pid": int64(11), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(time.Now().Add(-48 * time.Hour).Add(time.Second)), "uid": int64(9), "pid": int64(12), "domainId": "system", "status": 2},
		bson.M{"_id": bson.NewObjectIDFromTimestamp(time.Now().Add(-24 * time.Hour)), "uid": int64(9), "pid": int64(10), "domainId": "system", "status": 2},
	}
	if _, err := db.Collection("record").InsertMany(ctx, activityRecords); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "First"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2), "pid": "P2", "title": "Second"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(3), "pid": "P3", "title": "Hidden", "hidden": true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "uid": int64(8), "pid": int64(3), "domainId": "system", "status": 1,
	}); err != nil {
		t.Fatal(err)
	}
	problems, err := repo.AcceptedProblems(ctx, 7, 10)
	if err != nil || len(problems) != 2 || problems[0].DocID != 1 || problems[1].PID != "P2" {
		t.Fatalf("accepted problems: %+v err=%v", problems, err)
	}
	if hidden, err := repo.AcceptedProblems(ctx, 8, 10); err != nil || len(hidden) != 0 {
		t.Fatalf("hidden problems should be excluded: %+v err=%v", hidden, err)
	}
	if _, err := db.Collection("domain.user").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "uid": int64(7), "rp": int32(1500), "level": int32(3)},
		bson.M{"domainId": "system", "uid": int64(8), "rp": int64(1600), "level": int32(4)},
		bson.M{"domainId": "system", "uid": int64(9), "rp": int32(0)},
	}); err != nil {
		t.Fatal(err)
	}
	rp, level, err := repo.DomainStats(ctx, 7)
	if err != nil || rp != 1500 || level != 3 {
		t.Fatalf("domain stats: rp=%v level=%d err=%v", rp, level, err)
	}
	rank, err := repo.Rank(ctx, 7)
	if err != nil || rank != 2 {
		t.Fatalf("rank: %d err=%v", rank, err)
	}
	if rank, err := repo.Rank(ctx, 999); err != nil || rank != 0 {
		t.Fatalf("rank without rp: %d err=%v", rank, err)
	}
	ranking, total, err := repo.Ranking(ctx, 1, 10)
	if err != nil || total != 2 || len(ranking) != 2 || ranking[0].UID != 8 || ranking[0].RP != 1600 || ranking[1].UID != 7 {
		t.Fatalf("ranking: %+v total=%d err=%v", ranking, total, err)
	}
	activity, err := repo.Activity(ctx, 9)
	if err != nil {
		t.Fatal(err)
	}
	if activity.TotalSubmissions != 4 || activity.ActiveDays != 3 || activity.MaxStreak != 3 || activity.TotalSolved != 2 {
		t.Fatalf("activity: %+v", activity)
	}
}
