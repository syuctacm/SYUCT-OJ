package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
)

func TestMongoSessionSurvivesManagerRestart(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_session_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	first, err := NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatalf("first manager: %v", err)
	}
	token, err := first.IssueForUserContext(ctx, 42)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	var stored sessionDocument
	if err := db.Collection("session").FindOne(ctx, bson.M{}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.ID == token || len(stored.ID) != 64 {
		t.Fatalf("session stores raw token or unexpected id: %q", stored.ID)
	}

	second, err := NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatalf("second manager: %v", err)
	}
	session, ok := second.GetContext(ctx, token)
	if !ok || session.UserID != 42 || session.Kind != "user" {
		t.Fatalf("session after restart: %+v, ok=%v", session, ok)
	}
	if !second.ValidateContext(ctx, token) {
		t.Fatal("persisted session should validate")
	}
	second.RevokeContext(ctx, token)
	if first.ValidateContext(ctx, token) {
		t.Fatal("revoked session should be invalid across managers")
	}

	indexes, err := db.Collection("session").Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer indexes.Close(ctx)
	foundTTL := false
	for indexes.Next(ctx) {
		var index bson.M
		if err := indexes.Decode(&index); err != nil {
			t.Fatal(err)
		}
		if index["name"] == "session_ttl" {
			foundTTL = true
		}
	}
	if !foundTTL {
		t.Fatal("session_ttl index was not created")
	}
}

func TestMongoSessionRemovesExpiredToken(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_session_expiry_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	manager, err := NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	manager.now = func() time.Time { return now }
	token, err := manager.IssueContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	if manager.ValidateContext(ctx, token) {
		t.Fatal("expired persisted session should be invalid")
	}
	count, err := db.Collection("session").CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expired session was not deleted, count=%d", count)
	}
}
