package migration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/migration"
)

func newStatsTestDB(t *testing.T) (context.Context, *mongo.Database) {
	t.Helper()
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	name := fmt.Sprintf("syuct_oj_stats_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		cancel()
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
		cancel()
	})
	return ctx, db
}

func TestStatsRebuildMatchesHydroProblemStat(t *testing.T) {
	ctx, db := newStatsTestDB(t)
	normalContest := bson.NewObjectID()
	pretest, _ := bson.ObjectIDFromHex("000000000000000000000000")
	generate, _ := bson.ObjectIDFromHex("000000000000000000000001")

	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(101), "pid": "P101", "nSubmit": int64(999), "nAccept": int64(999)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(102), "pid": "P102", "nSubmit": int64(999), "nAccept": int64(999)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 11, "owner": int64(7), "vote": int64(3)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 11, "owner": int64(7), "vote": int64(2)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 11, "owner": int64(8), "vote": int64(0)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 11, "owner": int64(9), "vote": int64(1)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("domain.user").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "uid": int64(7), "nSubmit": int64(999), "nAccept": int64(999), "nLiked": int64(999)},
		bson.M{"domainId": "system", "uid": int64(8), "nSubmit": int64(999), "nAccept": int64(999), "nLiked": int64(999)},
		bson.M{"domainId": "system", "uid": int64(9), "nSubmit": int64(999), "nAccept": int64(999), "nLiked": int64(999)},
	}); err != nil {
		t.Fatal(err)
	}
	records := []any{
		bson.M{"uid": int64(7), "pid": int64(101), "status": int32(1), "score": int32(100), "contest": normalContest},
		bson.M{"uid": int64(7), "pid": int64(101), "status": int32(2), "score": int32(50), "contest": normalContest},
		bson.M{"uid": int64(7), "pid": int64(102), "status": int32(1), "score": int32(80), "contest": normalContest},
		bson.M{"uid": int64(8), "pid": int64(101), "status": int32(2), "score": int32(0), "contest": normalContest},
		bson.M{"uid": int64(8), "pid": int64(101), "status": int32(1), "score": int32(100), "contest": normalContest},
		bson.M{"uid": int64(8), "pid": int64(102), "status": int32(7), "score": int32(0), "contest": normalContest},
		bson.M{"uid": int64(7), "pid": int64(101), "status": int32(9), "score": int32(100), "contest": normalContest},
		bson.M{"uid": int64(7), "pid": int64(101), "status": int32(1), "score": int32(100), "contest": pretest},
		bson.M{"uid": int64(8), "pid": int64(102), "status": int32(1), "score": int32(100), "contest": generate},
		bson.M{"uid": int64(-1), "pid": int64(101), "status": int32(1), "score": int32(100), "contest": normalContest},
	}
	for _, record := range records {
		doc := record.(bson.M)
		doc["_id"] = bson.NewObjectID()
		doc["domainId"] = "system"
	}
	if _, err := db.Collection("record").InsertMany(ctx, records); err != nil {
		t.Fatal(err)
	}

	dry, err := migration.RunStatsRebuild(ctx, db, migration.StatsOptions{Domain: "system", DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Records != 7 || dry.SolutionDocs != 3 || dry.UserStatsMatched != 3 || dry.LikeStatsMatched != 2 || dry.ProblemsMatched != 2 {
		t.Fatalf("dry report = %+v", dry)
	}
	if dry.UserDocsUpdated != 0 || dry.ProblemDocsUpdated != 0 {
		t.Fatalf("dry report mutated counts = %+v", dry)
	}
	assertUserStats(t, ctx, db, 7, 999, 999, 999)
	assertUserStats(t, ctx, db, 8, 999, 999, 999)
	assertUserStats(t, ctx, db, 9, 999, 999, 999)

	done, err := migration.RunStatsRebuild(ctx, db, migration.StatsOptions{Domain: "system"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if done.UserDocsUpdated != 3 || done.ProblemDocsUpdated != 2 {
		t.Fatalf("execute report = %+v", done)
	}
	assertUserStats(t, ctx, db, 7, 2, 2, 5)
	assertUserStats(t, ctx, db, 8, 2, 1, 0)
	assertUserStats(t, ctx, db, 9, 0, 0, 1)

	problem101 := readProblemStats(t, ctx, db, 101)
	if problem101.NSubmit != 5 || problem101.NAccept != 3 || problem101.Stats["AC"] != 3 || problem101.Stats["WA"] != 2 {
		t.Fatalf("problem 101 = %+v", problem101)
	}
	if problem101.Stats["s0"] != 1 || problem101.Stats["s50"] != 1 || problem101.Stats["s100"] != 3 {
		t.Fatalf("problem 101 histogram = %+v", problem101.Stats)
	}
	problem102 := readProblemStats(t, ctx, db, 102)
	if problem102.NSubmit != 2 || problem102.NAccept != 1 || problem102.Stats["AC"] != 1 || problem102.Stats["CE"] != 1 {
		t.Fatalf("problem 102 = %+v", problem102)
	}
	if problem102.Stats["s0"] != 1 || problem102.Stats["s80"] != 1 {
		t.Fatalf("problem 102 histogram = %+v", problem102.Stats)
	}

	rerun, err := migration.RunStatsRebuild(ctx, db, migration.StatsOptions{Domain: "system"})
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if rerun.UserDocsUpdated != 0 || rerun.ProblemDocsUpdated != 0 {
		t.Fatalf("rerun report = %+v", rerun)
	}
}

func TestStatsRebuildReportsMissingProblems(t *testing.T) {
	ctx, db := newStatsTestDB(t)
	contest := bson.NewObjectID()
	if _, err := db.Collection("domain.user").InsertOne(ctx, bson.M{
		"domainId": "system", "uid": int64(7),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(7), "pid": int64(999),
		"status": int32(1), "score": int32(100), "contest": contest,
	}); err != nil {
		t.Fatal(err)
	}
	report, err := migration.RunStatsRebuild(ctx, db, migration.StatsOptions{Domain: "system", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.ProblemsMatched != 0 || report.ProblemsMissing != 1 || len(report.Warnings) == 0 {
		t.Fatalf("missing problem report = %+v", report)
	}
}

func TestStatsRebuildReportsMissingUsers(t *testing.T) {
	ctx, db := newStatsTestDB(t)
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(999), "pid": int64(1),
		"status": int32(1), "score": int32(100), "contest": bson.NewObjectID(),
	}); err != nil {
		t.Fatal(err)
	}
	report, err := migration.RunStatsRebuild(ctx, db, migration.StatsOptions{Domain: "system", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.UsersMissing != 1 || report.UserStatsMatched != 0 || len(report.Warnings) == 0 {
		t.Fatalf("missing user report = %+v", report)
	}
}

type storedProblemStats struct {
	NSubmit int64
	NAccept int64
	Stats   map[string]int64
}

func readProblemStats(t *testing.T, ctx context.Context, db *mongo.Database, pid int64) storedProblemStats {
	t.Helper()
	var doc struct {
		NSubmit int64  `bson:"nSubmit"`
		NAccept int64  `bson:"nAccept"`
		Stats   bson.M `bson:"stats"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"domainId": "system", "docType": 10, "docId": pid}).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	stats := map[string]int64{}
	for key, value := range doc.Stats {
		switch typed := value.(type) {
		case int32:
			stats[key] = int64(typed)
		case int64:
			stats[key] = typed
		case float64:
			stats[key] = int64(typed)
		}
	}
	return storedProblemStats{NSubmit: doc.NSubmit, NAccept: doc.NAccept, Stats: stats}
}

func assertUserStats(t *testing.T, ctx context.Context, db *mongo.Database, uid, submit, accepted, liked int64) {
	t.Helper()
	var user struct {
		NSubmit int64 `bson:"nSubmit"`
		NAccept int64 `bson:"nAccept"`
		NLiked  int64 `bson:"nLiked"`
	}
	if err := db.Collection("domain.user").FindOne(ctx, bson.M{"domainId": "system", "uid": uid}).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if user.NSubmit != submit || user.NAccept != accepted || user.NLiked != liked {
		t.Fatalf("uid %d stats = %+v", uid, user)
	}
}
