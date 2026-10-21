package discussion_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/discussion"
)

func TestDiscussionRepositoryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_discussion_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	firstID := bson.NewObjectID()
	secondID := bson.NewObjectID()
	_, err = db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 21, "docId": firstID, "title": "普通讨论", "owner": int64(1), "updateAt": time.Now().Add(-time.Hour), "nReply": int64(2), "views": int64(3), "parentType": int64(20), "parentId": "qa"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 21, "docId": secondID, "title": "置顶讨论", "owner": int64(2), "updateAt": time.Now().Add(-2 * time.Hour), "nReply": int64(4), "views": int64(5), "pin": true, "content": "# 内容", "parentType": int64(20), "parentId": "advice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := discussion.NewRepository(db)
	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 || items[0].Title != "置顶讨论" {
		t.Fatalf("unexpected discussions: %+v", items)
	}
	qa, err := repo.ListFiltered(ctx, 20, "qa")
	if err != nil || len(qa) != 1 || qa[0].Title != "普通讨论" {
		t.Fatalf("filter by node: %+v err=%v", qa, err)
	}
	advice, err := repo.ListFiltered(ctx, 20, "advice")
	if err != nil || len(advice) != 1 || advice[0].Title != "置顶讨论" {
		t.Fatalf("filter by advice node: %+v err=%v", advice, err)
	}
	if all, err := repo.ListFiltered(ctx, 20, ""); err != nil || len(all) != 2 {
		t.Fatalf("filter by parent type only: %+v err=%v", all, err)
	}
	if none, err := repo.ListFiltered(ctx, 30, ""); err != nil || len(none) != 0 {
		t.Fatalf("unexpected contests discussions: %+v err=%v", none, err)
	}
	found, err := repo.Get(ctx, secondID.Hex())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Title != "置顶讨论" || found.Content != "# 内容" {
		t.Fatalf("unexpected discussion: %+v", found)
	}
}
