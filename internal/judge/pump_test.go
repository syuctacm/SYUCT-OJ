package judge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/database"
)

func TestTaskLeaseAndResultLifecycle(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_pump_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	rid := bson.NewObjectID()
	taskID := bson.NewObjectID()
	_, err = db.Collection("task").InsertOne(ctx, bson.M{
		"_id": taskID, "type": "judge", "rid": rid, "priority": 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	pump := NewTaskPump(db, NewHub(nil))
	pump.leaseDuration = 200 * time.Millisecond
	leased, err := pump.leaseNext(ctx)
	if err != nil {
		t.Fatalf("lease task: %v", err)
	}
	if leased["_id"] != taskID || numericInt(leased["attempts"]) != 1 {
		t.Fatalf("unexpected first lease: %+v", leased)
	}
	if _, err := pump.leaseNext(ctx); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("active lease was offered twice: %v", err)
	}

	_, err = db.Collection("task").UpdateOne(ctx, bson.M{"_id": taskID}, bson.M{
		"$set": bson.M{"leaseUntil": time.Now().UTC().Add(-time.Second)},
	})
	if err != nil {
		t.Fatal(err)
	}
	released, err := pump.leaseNext(ctx)
	if err != nil {
		t.Fatalf("re-lease expired task: %v", err)
	}
	if numericInt(released["attempts"]) != 2 {
		t.Fatalf("attempts after retry: %+v", released)
	}

	handled := 0
	pump.SetResultHandler(func(payload map[string]any) error {
		handled++
		return nil
	})
	before := time.Now()
	if err := pump.HandleResult(ctx, map[string]any{"key": "next", "rid": rid.Hex()}); err != nil {
		t.Fatalf("handle next: %v", err)
	}
	var afterNext bson.M
	if err := db.Collection("task").FindOne(ctx, bson.M{"_id": taskID}).Decode(&afterNext); err != nil {
		t.Fatalf("task after next: %v", err)
	}
	leaseUntil, ok := afterNext["leaseUntil"].(bson.DateTime)
	if !ok || leaseUntil.Time().Before(before) {
		t.Fatalf("lease was not renewed: %+v", afterNext)
	}
	if err := pump.HandleResult(ctx, map[string]any{"key": "end", "rid": rid.Hex()}); err != nil {
		t.Fatalf("handle end: %v", err)
	}
	if handled != 2 {
		t.Fatalf("result handler calls = %d", handled)
	}
	if err := pump.HandleResult(ctx, map[string]any{"key": "end", "rid": rid.Hex()}); err != nil {
		t.Fatalf("duplicate end: %v", err)
	}
	if handled != 2 {
		t.Fatalf("duplicate result was applied: handler calls = %d", handled)
	}
	count, err := db.Collection("task").CountDocuments(ctx, bson.M{"_id": taskID})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("completed task was not removed")
	}
}

func TestTaskLeaseSurvivesResultPersistenceFailure(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_pump_failure_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	rid := bson.NewObjectID()
	taskID := bson.NewObjectID()
	if _, err := db.Collection("task").InsertOne(ctx, bson.M{
		"_id": taskID, "type": "judge", "rid": rid, "priority": 1,
	}); err != nil {
		t.Fatal(err)
	}
	pump := NewTaskPump(db, NewHub(nil))
	pump.SetResultHandler(func(map[string]any) error {
		return errors.New("database unavailable")
	})
	if err := pump.HandleResult(ctx, map[string]any{"key": "end", "rid": rid.Hex()}); err == nil {
		t.Fatal("expected persistence failure")
	}
	count, err := db.Collection("task").CountDocuments(ctx, bson.M{"_id": taskID})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("task was deleted despite result persistence failure")
	}
}

func numericInt(value any) int {
	switch number := value.(type) {
	case int32:
		return int(number)
	case int64:
		return int(number)
	case float64:
		return int(number)
	case int:
		return number
	default:
		return -1
	}
}
