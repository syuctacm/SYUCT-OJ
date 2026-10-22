package migration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/migration"
)

func TestFlattenDomains(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbName := fmt.Sprintf("syuct_oj_migration_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	docID := bson.NewObjectID()
	recordID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": docID, "domainId": "contest2025", "docType": 10, "title": "A",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("record").InsertOne(ctx, bson.M{
		"_id": recordID, "domainId": "contest2025", "pid": 1, "uid": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("domain.user").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "uid": int32(1), "nSubmit": int32(10), "nAccept": int32(5), "role": "default"},
		bson.M{"domainId": "contest2025", "uid": int32(1), "nSubmit": int32(2), "nAccept": int32(1), "role": "root"},
		bson.M{"domainId": "select2025", "uid": int32(2), "nSubmit": int32(3), "nAccept": int32(2), "role": "default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("domain").InsertOne(ctx, bson.M{
		"_id": "contest2025", "lower": "contest2025", "name": "25年校赛",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("storage").InsertMany(ctx, []any{
		bson.M{"_id": "a", "path": "problem/contest2025/1/testdata/1.in"},
		bson.M{"_id": "b", "path": "contest/contest2025/abc/public/a.txt"},
		bson.M{"_id": "c", "path": "training/select2025/def/lesson.pdf"},
		bson.M{"_id": "d", "path": "problem/system/2/testdata/2.in"},
		bson.M{"_id": "e", "path": "user/1/.avatar.png"},
	})
	if err != nil {
		t.Fatal(err)
	}

	opts := migration.Options{
		Sources: []string{"contest2025", "select2025"},
		Target:  migration.DefaultTargetDomain,
		DryRun:  true,
	}
	dryReport, err := migration.Run(ctx, db, opts)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !dryReport.DryRun || dryReport.UsersMerged != 0 {
		t.Fatalf("unexpected dry-run report: %+v", dryReport)
	}
	var untouched bson.M
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": docID}).Decode(&untouched); err != nil {
		t.Fatal(err)
	}
	if untouched["domainId"] != "contest2025" {
		t.Fatal("dry-run mutated document")
	}
	if countStoragePathsForTest(t, ctx, db, "contest2025") != 2 || countStoragePathsForTest(t, ctx, db, "select2025") != 1 {
		t.Fatal("dry-run mutated storage paths")
	}
	if item := findCollectionReport(dryReport, "storage.path", "contest2025"); item == nil || item.Matched != 2 {
		t.Fatalf("dry storage report = %+v", item)
	}

	opts.DryRun = false
	report, err := migration.Run(ctx, db, opts)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if report.UsersMerged != 2 {
		t.Fatalf("users merged = %d, want 2", report.UsersMerged)
	}
	if report.Validation == nil || !report.Validation.OK || len(report.Validation.Residuals) != 0 {
		t.Fatalf("migration validation failed: %+v", report.Validation)
	}
	if report.UsersRemaining == nil || *report.UsersRemaining != 0 {
		t.Fatalf("users remaining = %v, want 0", report.UsersRemaining)
	}
	if item := findCollectionReport(report, "storage.path", "contest2025"); item == nil || item.Modified != 2 || item.Remaining == nil || *item.Remaining != 0 {
		t.Fatalf("storage report = %+v", item)
	}

	var moved bson.M
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": docID}).Decode(&moved); err != nil {
		t.Fatal(err)
	}
	if moved["domainId"] != "system" {
		t.Fatalf("document domainId = %v", moved["domainId"])
	}
	var movedRecord bson.M
	if err := db.Collection("record").FindOne(ctx, bson.M{"_id": recordID}).Decode(&movedRecord); err != nil {
		t.Fatal(err)
	}
	if movedRecord["domainId"] != "system" {
		t.Fatalf("record domainId = %v", movedRecord["domainId"])
	}

	var user struct {
		UID     int64  `bson:"uid"`
		NSubmit int64  `bson:"nSubmit"`
		NAccept int64  `bson:"nAccept"`
		Role    string `bson:"role"`
	}
	if err := db.Collection("domain.user").FindOne(ctx, bson.M{"domainId": "system", "uid": int32(1)}).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if user.NSubmit != 12 || user.NAccept != 6 {
		t.Fatalf("merged user stats = %+v", user)
	}
	if user.Role != "default" {
		t.Fatalf("target role overwritten: %q", user.Role)
	}
	count, err := db.Collection("domain.user").CountDocuments(ctx, bson.M{"domainId": bson.M{"$ne": "system"}})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("source domain.user documents remain: %d", count)
	}
	legacyCount, err := db.Collection("domain.legacy").CountDocuments(ctx, bson.M{"_id": "contest2025"})
	if err != nil {
		t.Fatal(err)
	}
	if legacyCount != 1 {
		t.Fatalf("legacy domain count = %d", legacyCount)
	}

	paths := map[string]string{}
	cursor, err := db.Collection("storage").Find(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var item struct {
			ID   string `bson:"_id"`
			Path string `bson:"path"`
		}
		if err := cursor.Decode(&item); err != nil {
			t.Fatal(err)
		}
		paths[item.ID] = item.Path
	}
	if err := cursor.Err(); err != nil {
		t.Fatal(err)
	}
	if paths["a"] != "problem/system/1/testdata/1.in" || paths["b"] != "contest/system/abc/public/a.txt" || paths["c"] != "training/system/def/lesson.pdf" {
		t.Fatalf("migrated storage paths = %+v", paths)
	}
	if paths["d"] != "problem/system/2/testdata/2.in" || paths["e"] != "user/1/.avatar.png" {
		t.Fatalf("unrelated storage paths changed: %+v", paths)
	}

	secondOpts := opts
	secondOpts.DryRun = true
	secondReport, err := migration.Run(ctx, db, secondOpts)
	if err != nil {
		t.Fatalf("second dry run: %v", err)
	}
	for _, item := range secondReport.Collections {
		if item.Matched != 0 {
			t.Fatalf("migration is not idempotent: %+v", item)
		}
	}
	if secondReport.UsersMerged != 0 {
		t.Fatalf("second dry run users merged = %d", secondReport.UsersMerged)
	}
}

func countStoragePathsForTest(t *testing.T, ctx context.Context, db *mongo.Database, source string) int64 {
	t.Helper()
	pattern := "^(problem|contest|training)/" + source + "/"
	count, err := db.Collection("storage").CountDocuments(ctx, bson.M{"path": bson.M{"$regex": pattern}})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func findCollectionReport(report *migration.Report, collection, domain string) *migration.CollectionReport {
	for i := range report.Collections {
		if report.Collections[i].Collection == collection && report.Collections[i].Domain == domain {
			return &report.Collections[i]
		}
	}
	return nil
}

func TestFlattenRenamesConflictingProblemPIDs(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_migration_pid_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "System"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 10, "docId": int64(2), "pid": "P1", "title": "Contest"},
	}); err != nil {
		t.Fatal(err)
	}

	report, err := migration.Run(ctx, db, migration.Options{
		Sources: []string{"contest2025"},
		Target:  migration.DefaultTargetDomain,
		DryRun:  false,
	})
	if err != nil {
		t.Fatalf("migration: %v", err)
	}
	if len(report.ProblemsRenamed) != 1 {
		t.Fatalf("renames = %#v", report.ProblemsRenamed)
	}
	rename := report.ProblemsRenamed[0]
	if rename.From != "P1" || rename.To != "P2" || rename.DocID != 2 {
		t.Fatalf("rename = %#v", rename)
	}
	var contest struct {
		PID        string   `bson:"pid"`
		LegacyPIDs []string `bson:"legacyPids"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"domainId": "system", "docType": 10, "docId": int64(2)}).Decode(&contest); err != nil {
		t.Fatal(err)
	}
	if contest.PID != "P2" || len(contest.LegacyPIDs) != 1 || contest.LegacyPIDs[0] != "P1" {
		t.Fatalf("contest problem = %#v", contest)
	}
	var system struct {
		PID string `bson:"pid"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"domainId": "system", "docType": 10, "docId": int64(1)}).Decode(&system); err != nil {
		t.Fatal(err)
	}
	if system.PID != "P1" {
		t.Fatalf("system pid changed: %q", system.PID)
	}
}

func TestFlattenRefusesProblemDocIDCollision(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_migration_collision_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "System"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 10, "docId": int64(1), "pid": "C1", "title": "Contest"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("domain").InsertOne(ctx, bson.M{"_id": "contest2025", "name": "contest2025"}); err != nil {
		t.Fatal(err)
	}

	dry, err := migration.Run(ctx, db, migration.Options{
		Sources: []string{"contest2025"},
		Target:  migration.DefaultTargetDomain,
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(dry.Conflicts) != 1 || dry.Conflicts[0].Kind != "docId" {
		t.Fatalf("dry conflicts = %#v", dry.Conflicts)
	}

	report, err := migration.Run(ctx, db, migration.Options{
		Sources: []string{"contest2025"},
		Target:  migration.DefaultTargetDomain,
		DryRun:  false,
	})
	if err == nil {
		t.Fatal("execute accepted a docId collision")
	}
	if report == nil || len(report.Conflicts) != 1 {
		t.Fatalf("report = %#v", report)
	}
	if count, _ := db.Collection("document").CountDocuments(ctx, bson.M{"domainId": "contest2025"}); count != 1 {
		t.Fatalf("source documents were modified: %d", count)
	}
	if count, _ := db.Collection("domain").CountDocuments(ctx, bson.M{"_id": "contest2025"}); count != 1 {
		t.Fatalf("source domain was archived")
	}
}

func TestFlattenRenumbersCollidingProblemDocIDs(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_migration_renumber_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	fileRoot := t.TempDir()
	oldDir := filepath.Join(fileRoot, "problem", "contest2025", "1", "testdata")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "1.in"), []byte("1 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "System"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 10, "docId": int32(1), "pid": "C1", "title": "Contest"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 10, "docId": int64(5), "pid": "C5", "title": "Contest 5"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 11, "parentType": 10, "parentId": int32(1), "owner": int64(7), "content": "solution"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 21, "parentType": 10, "parentId": int64(1), "title": "discussion"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 30, "docId": bson.NewObjectID(), "pids": []any{int32(1), int64(5)}},
		bson.M{"_id": bson.NewObjectID(), "domainId": "contest2025", "docType": 40, "docId": bson.NewObjectID(),
			"dag": []any{
				bson.M{"pids": []any{int32(1)}},
				bson.M{"pids": []any{int64(5), int64(1)}},
			}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "contest2025", "pid": int32(1), "uid": int64(7), "status": 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record.stat").InsertOne(ctx, bson.M{
		"domainId": "contest2025", "pid": int64(1), "uid": int64(7),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document.status").InsertOne(ctx, bson.M{
		"domainId": "contest2025", "docType": 10, "docId": int32(1), "uid": int64(7), "star": true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("storage").InsertMany(ctx, []any{
		bson.M{"_id": "old", "path": "problem/contest2025/1/testdata/1.in"},
		bson.M{"_id": "keep", "path": "problem/contest2025/5/testdata/5.in"},
	}); err != nil {
		t.Fatal(err)
	}

	report, err := migration.Run(ctx, db, migration.Options{
		Sources:        []string{"contest2025"},
		Target:         migration.DefaultTargetDomain,
		DryRun:         false,
		RenumberDocIDs: true,
		FileRoot:       fileRoot,
	})
	if err != nil {
		t.Fatalf("migration: %v", err)
	}
	if len(report.ProblemsRenumbered) != 1 {
		t.Fatalf("renumbered = %#v", report.ProblemsRenumbered)
	}
	rename := report.ProblemsRenumbered[0]
	if rename.OldDocID != 1 || rename.NewDocID != 6 {
		t.Fatalf("rename = %#v", rename)
	}

	var problem struct {
		DocID     int64 `bson:"docId"`
		LegacyDoc int64 `bson:"legacyDocId"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"domainId": "system", "docType": 10, "pid": "C1"}).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	if problem.DocID != 6 || problem.LegacyDoc != 1 {
		t.Fatalf("problem = %#v", problem)
	}

	var record struct {
		PID int64 `bson:"pid"`
	}
	if err := db.Collection("record").FindOne(ctx, bson.M{"domainId": "system", "uid": int64(7)}).Decode(&record); err != nil {
		t.Fatal(err)
	}
	if record.PID != 6 {
		t.Fatalf("record pid = %d", record.PID)
	}
	var status struct {
		DocID int64 `bson:"docId"`
	}
	if err := db.Collection("document.status").FindOne(ctx, bson.M{"domainId": "system", "uid": int64(7)}).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.DocID != 6 {
		t.Fatalf("status docId = %d", status.DocID)
	}
	var parents []struct {
		DocType  int   `bson:"docType"`
		ParentID int64 `bson:"parentId"`
	}
	cursor, err := db.Collection("document").Find(ctx, bson.M{"domainId": "system", "parentType": 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := cursor.All(ctx, &parents); err != nil {
		t.Fatal(err)
	}
	if len(parents) != 2 || parents[0].ParentID != 6 || parents[1].ParentID != 6 {
		t.Fatalf("parents = %#v", parents)
	}
	var contest struct {
		Pids []int64 `bson:"pids"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"domainId": "system", "docType": 30}).Decode(&contest); err != nil {
		t.Fatal(err)
	}
	if len(contest.Pids) != 2 || contest.Pids[0] != 6 || contest.Pids[1] != 5 {
		t.Fatalf("contest pids = %#v", contest.Pids)
	}
	var training struct {
		Dag []struct {
			Pids []int64 `bson:"pids"`
		} `bson:"dag"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"domainId": "system", "docType": 40}).Decode(&training); err != nil {
		t.Fatal(err)
	}
	if len(training.Dag) != 2 || training.Dag[0].Pids[0] != 6 || training.Dag[1].Pids[0] != 5 || training.Dag[1].Pids[1] != 6 {
		t.Fatalf("training dag = %#v", training.Dag)
	}
	var storage struct {
		Path string `bson:"path"`
	}
	if err := db.Collection("storage").FindOne(ctx, bson.M{"_id": "old"}).Decode(&storage); err != nil {
		t.Fatal(err)
	}
	if storage.Path != "problem/system/6/testdata/1.in" {
		t.Fatalf("storage path = %q", storage.Path)
	}
	if _, err := os.Stat(filepath.Join(fileRoot, "problem", "contest2025", "6", "testdata", "1.in")); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fileRoot, "problem", "contest2025", "1")); !os.IsNotExist(err) {
		t.Fatalf("old file dir still exists: %v", err)
	}
}
