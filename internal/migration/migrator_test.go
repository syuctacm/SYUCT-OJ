package migration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

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

	opts.DryRun = false
	report, err := migration.Run(ctx, db, opts)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if report.UsersMerged != 2 {
		t.Fatalf("users merged = %d, want 2", report.UsersMerged)
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
}
