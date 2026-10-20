package training_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/training"
)

func TestTrainingRepositoryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_training_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 40, "docId": firstID, "title": "基础训练", "description": "基础", "pin": false},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 40, "docId": secondID, "title": "置顶训练", "description": "置顶", "pin": true, "content": "# 内容"},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := training.NewRepository(db)
	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 || items[0].Title != "置顶训练" {
		t.Fatalf("unexpected trainings: %+v", items)
	}
	found, err := repo.Get(ctx, secondID.Hex())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Title != "置顶训练" || found.Content != "# 内容" {
		t.Fatalf("unexpected training: %+v", found)
	}
}
