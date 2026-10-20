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
