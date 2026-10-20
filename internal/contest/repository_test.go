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
			"attend": int64(20), "pids": []int64{3},
		},
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
	found, err := repo.Get(ctx, newID.Hex())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Title != "New Contest" || len(found.PIDs) != 1 {
		t.Fatalf("unexpected contest: %+v", found)
	}
	if _, err := repo.Get(ctx, "invalid"); err != contest.ErrNotFound {
		t.Fatalf("invalid id error = %v", err)
	}
}
