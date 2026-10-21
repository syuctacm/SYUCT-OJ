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
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 40, "docId": firstID, "title": "基础训练", "description": "基础", "pin": int32(0)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 40, "docId": secondID, "title": "置顶训练", "description": "置顶", "pin": int64(1), "content": "# 内容"},
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
	items, _, err = repo.ListFiltered(ctx, "置顶", 1, 20)
	if err != nil || len(items) != 1 || items[0].Title != "置顶训练" {
		t.Fatalf("training search: items=%+v err=%v", items, err)
	}
	trainingPage1, trainingTotal, err := repo.ListFiltered(ctx, "", 1, 1)
	if err != nil || trainingTotal != 2 || len(trainingPage1) != 1 || trainingPage1[0].Title != "置顶训练" {
		t.Fatalf("training page 1: items=%+v total=%d err=%v", trainingPage1, trainingTotal, err)
	}
	trainingPage2, trainingTotal, err := repo.ListFiltered(ctx, "", 2, 1)
	if err != nil || trainingTotal != 2 || len(trainingPage2) != 1 || trainingPage2[0].Title != "基础训练" {
		t.Fatalf("training page 2: items=%+v total=%d err=%v", trainingPage2, trainingTotal, err)
	}
	found, err := repo.Get(ctx, secondID.Hex())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Title != "置顶训练" || found.Content != "# 内容" {
		t.Fatalf("unexpected training: %+v", found)
	}

	detailID := bson.NewObjectID()
	if _, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 40, "docId": detailID,
		"title": "详情训练", "owner": int64(1), "attend": int64(1), "pin": int32(0),
		"content": "# 训练内容", "description": "**描述**",
		"dag": []any{
			bson.M{
				"_id": int64(1), "title": "第一节", "content": "章节内容",
				"requireNids": []any{}, "pids": []any{int64(201), int64(202)},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(201), "pid": "A1", "title": "题目一", "difficulty": 3, "nSubmit": int64(10), "nAccept": int64(5)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(202), "pid": "A2", "title": "题目二", "difficulty": 0, "nSubmit": int64(4), "nAccept": int64(1)},
	}); err != nil {
		t.Fatal(err)
	}
	olderRID := bson.NewObjectID()
	newerRID := bson.NewObjectID()
	if _, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": olderRID, "uid": int64(7), "pid": int64(201), "status": 2, "score": 0},
		bson.M{"_id": newerRID, "uid": int64(7), "pid": int64(201), "status": 1, "score": 100},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Collection("document.status").InsertOne(ctx, bson.M{
		"domainId": "system", "docType": 40, "docId": detailID, "uid": int64(7), "enroll": 1,
	}); err != nil {
		t.Fatal(err)
	}
	detail, err := repo.Detail(ctx, detailID.Hex(), 7)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if !detail.Enrolled || detail.DoneProblems != 1 || detail.TotalProblems != 2 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if len(detail.EnrolledUsers) != 1 || detail.EnrolledUsers[0] != 7 {
		t.Fatalf("unexpected enrolled users: %+v", detail.EnrolledUsers)
	}
	if len(detail.Sections) != 1 || detail.Sections[0].Status != "progress" {
		t.Fatalf("unexpected sections: %+v", detail.Sections)
	}
	problems := detail.Sections[0].Problems
	if len(problems) != 2 {
		t.Fatalf("unexpected problems: %+v", problems)
	}
	if problems[0].RID != newerRID.Hex() || problems[0].Status != 1 || !problems[0].Solved || !problems[0].Tried {
		t.Fatalf("unexpected latest record mapping: %+v", problems[0])
	}
	if problems[1].RID != "" || problems[1].Tried || problems[1].Solved {
		t.Fatalf("unexpected problem without record: %+v", problems[1])
	}
	anonymous, err := repo.Detail(ctx, detailID.Hex(), 0)
	if err != nil {
		t.Fatalf("anonymous detail: %v", err)
	}
	if anonymous.Enrolled || anonymous.Sections[0].Problems[0].RID != "" {
		t.Fatalf("unexpected anonymous detail: %+v", anonymous)
	}
}
