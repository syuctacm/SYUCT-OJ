package webauthn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	wa "github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func newWebAuthnTestService(t *testing.T) (*Service, *mongo.Database, context.Context) {
	t.Helper()
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	name := fmt.Sprintf("syuct_oj_webauthn_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		cancel()
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
		cancel()
	})
	service, err := New(ctx, db, user.NewRepository(db), Config{
		RPID: "localhost", RPDisplayName: "SYUCT OJ", RPOrigins: []string{"http://localhost:5174"},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return service, db, ctx
}

func seedWebAuthnUser(t *testing.T, db *mongo.Database, ctx context.Context, uid int64, username string) {
	t.Helper()
	hash := user.HashPassword("secret", "salt")
	_, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": uid, "uname": username, "unameLower": username,
		"mail": username + "@example.com", "mailLower": username + "@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func TestChallengeIsBoundAndSingleUse(t *testing.T) {
	service, _, ctx := newWebAuthnTestService(t)
	token, err := service.saveChallenge(ctx, 7, "register", &wa.SessionData{Challenge: "challenge", UserID: []byte("user-7")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.consumeChallenge(ctx, token, 8, "register"); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("cross-user challenge = %v", err)
	}
	if _, err := service.consumeChallenge(ctx, token, 7, "login"); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("cross-purpose challenge = %v", err)
	}
	session, err := service.consumeChallenge(ctx, token, 7, "register")
	if err != nil {
		t.Fatalf("consume challenge: %v", err)
	}
	if session.Challenge != "challenge" || string(session.UserID) != "user-7" {
		t.Fatalf("session = %+v", session)
	}
	if _, err := service.consumeChallenge(ctx, token, 7, "register"); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("reused challenge = %v", err)
	}
}

func TestExpiredChallengeIsRejected(t *testing.T) {
	service, _, ctx := newWebAuthnTestService(t)
	_, err := service.challenges.InsertOne(ctx, challengeDocument{
		ID: "expired", UID: 7, Purpose: "register", Data: `{"challenge":"expired"}`,
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.consumeChallenge(ctx, "expired", 7, "register"); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("expired challenge = %v", err)
	}
}

func TestBeginLoginRequiresCredentialWithoutCreatingHandle(t *testing.T) {
	service, db, ctx := newWebAuthnTestService(t)
	seedWebAuthnUser(t, db, ctx, 7, "alice")
	if _, _, err := service.BeginLogin(ctx, "alice"); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("begin login without credentials = %v", err)
	}
	var stored struct {
		WebAuthnID []byte `bson:"webauthnId"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.WebAuthnID) != 0 {
		t.Fatalf("login created a WebAuthn handle: %x", stored.WebAuthnID)
	}
}

func TestWebAuthnHandleIsStable(t *testing.T) {
	service, db, ctx := newWebAuthnTestService(t)
	seedWebAuthnUser(t, db, ctx, 7, "alice")
	if _, _, err := service.BeginRegistration(ctx, 7); err != nil {
		t.Fatal(err)
	}
	first, err := service.users.EnsureWebAuthnID(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.BeginRegistration(ctx, 7); err != nil {
		t.Fatal(err)
	}
	second, err := service.users.EnsureWebAuthnID(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || string(first) != string(second) {
		t.Fatalf("unstable WebAuthn handle: %x != %x", first, second)
	}
}

func TestCredentialOwnershipAndLifecycle(t *testing.T) {
	service, db, ctx := newWebAuthnTestService(t)
	seedWebAuthnUser(t, db, ctx, 7, "alice")
	credential := &wa.Credential{ID: []byte{1, 2, 3, 4}, AttestationFormat: "none"}
	if err := service.saveCredential(ctx, 7, "  My Key  ", credential); err != nil {
		t.Fatal(err)
	}
	items, err := service.ListCredentials(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "My Key" {
		t.Fatalf("credentials = %+v", items)
	}
	if err := service.saveCredential(ctx, 8, "stolen", credential); !errors.Is(err, ErrCredentialExists) {
		t.Fatalf("cross-user credential save = %v", err)
	}
	if err := service.DeleteCredential(ctx, 8, items[0].ID); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("cross-user delete = %v", err)
	}
	if err := service.DeleteCredential(ctx, 7, items[0].ID); err != nil {
		t.Fatal(err)
	}
	items, err = service.ListCredentials(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("credentials after delete = %+v", items)
	}
	if _, _, err := service.BeginLogin(ctx, "alice"); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("begin login after deleting the last credential = %v", err)
	}
}

func TestNormalizeName(t *testing.T) {
	if got := normalizeName("  "); got != "Passkey" {
		t.Fatalf("empty name = %q", got)
	}
	if got := normalizeName("  Phone  "); got != "Phone" {
		t.Fatalf("trimmed name = %q", got)
	}
	long := make([]rune, 70)
	for i := range long {
		long[i] = '镜'
	}
	if got := []rune(normalizeName(string(long))); len(got) != 64 {
		t.Fatalf("long name length = %d", len(got))
	}
}
