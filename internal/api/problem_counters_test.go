package api_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
)

func TestProblemSubmissionCounters(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_problem_counters_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1),
		"pid": "P1001", "title": "Counters", "owner": int64(1),
		"config": "type: default\ntime: 1000ms\nmemory: 256m\n",
		"data":   []any{bson.M{"_id": "problem/system/1/testdata/1.in", "name": "1.in", "size": int64(3)}},
	}); err != nil {
		t.Fatal(err)
	}
	problems := problem.NewRepository(db)
	submissions := submission.NewRepository(db, problems)

	read := func() (int64, int64) {
		var doc struct {
			NSubmit int64 `bson:"nSubmit"`
			NAccept int64 `bson:"nAccept"`
		}
		if err := db.Collection("document").FindOne(ctx, bson.M{"docType": 10, "docId": int64(1)}).Decode(&doc); err != nil {
			t.Fatal(err)
		}
		return doc.NSubmit, doc.NAccept
	}

	record, err := submissions.Create(ctx, 7, "P1001", "cpp", "int main(){}")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if submit, accept := read(); submit != 1 || accept != 0 {
		t.Fatalf("after submit = %d/%d", submit, accept)
	}

	apply := func(status int) {
		if err := submissions.ApplyJudgeResult(ctx, map[string]any{
			"rid": record.ID.Hex(), "status": status, "score": 100, "time": 10, "memory": 1024,
		}); err != nil {
			t.Fatalf("apply result %d: %v", status, err)
		}
	}
	apply(submission.StatusAccepted)
	if submit, accept := read(); submit != 1 || accept != 1 {
		t.Fatalf("after AC = %d/%d", submit, accept)
	}
	apply(2) // Wrong Answer: a rejudge leaves the accept counter.
	if submit, accept := read(); submit != 1 || accept != 0 {
		t.Fatalf("after WA rejudge = %d/%d", submit, accept)
	}
	apply(submission.StatusAccepted)
	if submit, accept := read(); submit != 1 || accept != 1 {
		t.Fatalf("after re-AC = %d/%d", submit, accept)
	}

	if _, err := submissions.Create(ctx, 7, "P1001", "cpp", "int main(){return 0;}"); err != nil {
		t.Fatalf("second create: %v", err)
	}
	if submit, accept := read(); submit != 2 || accept != 1 {
		t.Fatalf("after second submit = %d/%d", submit, accept)
	}
}
