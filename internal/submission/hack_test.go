package submission

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

func TestAppendHackCase(t *testing.T) {
	config := `type: default
time: 1000ms
memory: 256m
checker_type: testlib
checker: chk.cc
validator: val.cc
subtasks:
  - score: 100
    type: min
    cases:
      - input: 1.in
        output: 1.out
`
	updated, err := appendHackCase(config, "hack-abc.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"checker_type: testlib", "validator: val.cc", "input: hack-abc.in", "output: /dev/null", "input: 1.in"} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("updated config missing %q:\n%s", expected, updated)
		}
	}
	repeated, err := appendHackCase(updated, "hack-abc.in")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(repeated, "input: hack-abc.in") != 1 {
		t.Fatalf("appendHackCase is not idempotent:\n%s", repeated)
	}
}

func TestHackWorkflowIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_hack_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	config := `type: default
time: 1000ms
memory: 256m
checker_type: testlib
checker: chk.cc
validator: val.cc
subtasks:
  - score: 100
    type: min
    cases:
      - input: 1.in
        output: 1.out
`
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
		"docId": int64(9001), "pid": "PHACK", "title": "Hackable A+B", "owner": int64(1),
		"nSubmit": int64(0), "nAccept": int64(0), "hidden": false,
		"config": config,
		"data": []bson.M{
			{"_id": "problem/system/9001/testdata/1.in", "name": "1.in", "size": int64(6)},
			{"_id": "problem/system/9001/testdata/1.out", "name": "1.out", "size": int64(2)},
			{"_id": "problem/system/9001/testdata/chk.cc", "name": "chk.cc", "size": int64(10)},
			{"_id": "problem/system/9001/testdata/val.cc", "name": "val.cc", "size": int64(10)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	problems := problem.NewRepository(db)
	repo := NewRepository(db, problems)
	store := judge.NewFileStore(t.TempDir())
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	repo.SetFileStore(store)

	target, err := repo.Create(ctx, 2, "PHACK", "cpp", "int main(){int a,b;std::cin>>a>>b;std::cout<<a+b;}", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").UpdateOne(ctx, bson.M{"_id": target.ID}, bson.M{"$set": bson.M{"status": StatusAccepted, "contest": nil}}); err != nil {
		t.Fatal(err)
	}
	own, err := repo.Create(ctx, 1, "PHACK", "cpp", "int main(){long long a,b;std::cin>>a>>b;std::cout<<a+b;}", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").UpdateOne(ctx, bson.M{"_id": own.ID}, bson.M{"$set": bson.M{"status": StatusAccepted}}); err != nil {
		t.Fatal(err)
	}
	if err := problems.IncStats(ctx, 9001, 0, 2); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.CreateHack(ctx, 1, "PHACK", own.ID.Hex(), []byte("1 2\n")); err == nil || !strings.Contains(err.Error(), "your own") {
		t.Fatalf("self hack error = %v", err)
	}
	if _, err := repo.CreateHack(ctx, 3, "PHACK", target.ID.Hex(), []byte("1 2\n")); err == nil || !strings.Contains(err.Error(), "must accept") {
		t.Fatalf("no accepted solution error = %v", err)
	}

	hack, err := repo.CreateHack(ctx, 1, "PHACK", target.ID.Hex(), []byte("2000000000 2000000000\n"))
	if err != nil {
		t.Fatal(err)
	}
	if hack.Type != "hack" || hack.HackTarget == nil || *hack.HackTarget != target.ID || hack.Files["hack"] == "" {
		t.Fatalf("hack record = %+v", hack)
	}
	var task bson.M
	if err := db.Collection("task").FindOne(ctx, bson.M{"rid": hack.ID}).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if task["type"] != "hack" || task["files"] == nil {
		t.Fatalf("hack task = %+v", task)
	}

	if err := repo.ApplyJudgeResult(ctx, map[string]any{
		"rid": hack.ID.Hex(), "status": StatusHackSuccessful, "score": 0,
	}); err != nil {
		t.Fatal(err)
	}
	updatedProblem, err := problems.GetForEditor(ctx, "PHACK")
	if err != nil {
		t.Fatal(err)
	}
	inputName := "hack-" + hack.ID.Hex() + ".in"
	if !strings.Contains(fmt.Sprint(updatedProblem.Config), inputName) {
		t.Fatalf("hack case was not appended: %s", updatedProblem.Config)
	}
	foundFile := false
	for _, file := range updatedProblem.Data {
		if file.Name == inputName {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Fatalf("hack testdata metadata missing: %+v", updatedProblem.Data)
	}
	input, err := store.OpenProblemFile("system", "9001", inputName)
	if err != nil {
		t.Fatal(err)
	}
	input.Close()

	taskCount, err := db.Collection("task").CountDocuments(ctx, bson.M{"meta.hackRejudge": inputName})
	if err != nil {
		t.Fatal(err)
	}
	if taskCount != 2 {
		t.Fatalf("hack rejudge task count = %d, want 2", taskCount)
	}
	var rejudgeTask bson.M
	if err := db.Collection("task").FindOne(ctx, bson.M{"meta.hackRejudge": inputName}).Decode(&rejudgeTask); err != nil {
		t.Fatal(err)
	}
	files, _ := rejudgeTask["data"].(bson.A)
	foundTaskFile := false
	for _, raw := range files {
		encoded, err := bson.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		var file struct {
			Name string `bson:"name"`
		}
		if err := bson.Unmarshal(encoded, &file); err != nil {
			t.Fatal(err)
		}
		if file.Name == inputName {
			foundTaskFile = true
			break
		}
	}
	if !foundTaskFile {
		t.Fatalf("hack rejudge task data missing %s: %+v", inputName, rejudgeTask["data"])
	}
	for _, rid := range []bson.ObjectID{target.ID, own.ID} {
		var current struct {
			Status int `bson:"status"`
		}
		if err := db.Collection("record").FindOne(ctx, bson.M{"_id": rid}).Decode(&current); err != nil {
			t.Fatal(err)
		}
		if current.Status != StatusWaiting {
			t.Fatalf("rejudge target %s status = %d, want waiting", rid.Hex(), current.Status)
		}
	}

	if err := repo.ApplyJudgeResult(ctx, map[string]any{"rid": target.ID.Hex(), "status": StatusHacked, "score": 0}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyJudgeResult(ctx, map[string]any{"rid": own.ID.Hex(), "status": StatusAccepted, "score": 100}); err != nil {
		t.Fatal(err)
	}
	targetRecord, err := repo.Get(ctx, target.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	ownRecord, err := repo.Get(ctx, own.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if targetRecord.Status != StatusHacked || ownRecord.Status != StatusAccepted {
		t.Fatalf("rejudge statuses = target %d, own %d", targetRecord.Status, ownRecord.Status)
	}
	finalProblem, err := problems.GetForEditor(ctx, "PHACK")
	if err != nil {
		t.Fatal(err)
	}
	if finalProblem.NAccept != 1 {
		t.Fatalf("nAccept after hack = %d, want 1", finalProblem.NAccept)
	}
}
