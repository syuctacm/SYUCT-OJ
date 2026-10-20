package user_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestRepositoryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_user_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	_, err = db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "Alice", "unameLower": "alice",
		"mail": "Alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1), "bio": "ACMer",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := user.NewRepository(db)
	found, err := repo.FindByLogin(ctx, "ALICE")
	if err != nil {
		t.Fatalf("find by login: %v", err)
	}
	if found.ID != 7 || found.Uname != "Alice" {
		t.Fatalf("unexpected user: %+v", found)
	}
	found, err = repo.FindByID(ctx, 7)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if user.VerifyPassword(found, "secret") != nil {
		t.Fatal("stored password cannot be verified")
	}
	if found.Bio != "ACMer" {
		t.Fatalf("bio = %q", found.Bio)
	}
	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(1), "domainId": "system", "status": 2},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(1), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(2), "domainId": "system", "status": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	submit, accepted, err := repo.Statistics(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if submit != 3 || accepted != 2 {
		t.Fatalf("statistics submit=%d accepted=%d", submit, accepted)
	}
}
