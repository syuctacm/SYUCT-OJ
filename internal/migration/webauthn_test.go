package migration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	wa "github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/migration"
)

func newWebAuthnMigrationDB(t *testing.T) (context.Context, *mongo.Database) {
	t.Helper()
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	name := fmt.Sprintf("syuct_oj_webauthn_migration_test_%d", time.Now().UnixNano())
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
	return ctx, db
}

func TestLegacyWebAuthnImportDryRunExecuteAndRerun(t *testing.T) {
	ctx, db := newWebAuthnMigrationDB(t)
	credentialID := []byte{1, 2, 3, 4, 5}
	publicKey := []byte{0xa1, 0x01, 0x02, 0x03}
	_, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice",
		"authenticators": []any{bson.M{
			"name": "Legacy Key", "regat": int64(1700000000000), "fmt": "none",
			"counter": int64(3), "aaguid": "00112233-4455-6677-8899-aabbccddeeff",
			"credentialID":        bson.Binary{Subtype: 0, Data: credentialID},
			"credentialPublicKey": bson.Binary{Subtype: 0, Data: publicKey},
			"userVerified":        true, "credentialDeviceType": "multiDevice", "credentialBackedUp": true,
			"authenticatorAttachment": "platform", "transports": []string{"internal", "hybrid"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("domain.user").InsertOne(ctx, bson.M{
		"domainId": "contest2025", "uid": int64(7), "role": "default",
	})
	if err != nil {
		t.Fatal(err)
	}

	dry, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{
		Sources: []string{"contest2025"}, DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !dry.OK || dry.Users != 1 || dry.Credentials != 1 || dry.PlannedImports != 1 || dry.PlannedUserUpdates != 1 {
		t.Fatalf("dry report = %+v", dry)
	}
	if len(dry.Domains) != 1 || dry.Domains[0].Users != 1 || dry.Domains[0].Credentials != 1 {
		t.Fatalf("dry domain report = %+v", dry.Domains)
	}
	if count, err := db.Collection("webauthn_credentials").CountDocuments(ctx, bson.M{}); err != nil || count != 0 {
		t.Fatalf("dry run wrote credentials: count=%d err=%v", count, err)
	}

	executed, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{
		Sources: []string{"contest2025"}, DryRun: false,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !executed.OK || executed.Imported != 1 || executed.UsersUpdated != 1 || executed.Existing != 0 {
		t.Fatalf("execute report = %+v", executed)
	}

	id := base64.RawURLEncoding.EncodeToString(credentialID)
	var stored struct {
		UID       int64     `bson:"uid"`
		Name      string    `bson:"name"`
		Data      string    `bson:"data"`
		CreatedAt time.Time `bson:"createdAt"`
	}
	if err := db.Collection("webauthn_credentials").FindOne(ctx, bson.M{"_id": id}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.UID != 7 || stored.Name != "Legacy Key" || stored.CreatedAt.UnixMilli() != 1700000000000 {
		t.Fatalf("stored credential = %+v", stored)
	}
	var credential wa.Credential
	if err := json.Unmarshal([]byte(stored.Data), &credential); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(credential.ID, credentialID) || !bytes.Equal(credential.PublicKey, publicKey) {
		t.Fatalf("credential bytes = %+v", credential)
	}
	if credential.Authenticator.SignCount != 3 || len(credential.Authenticator.AAGUID) != 16 || credential.Authenticator.Attachment != "platform" {
		t.Fatalf("authenticator = %+v", credential.Authenticator)
	}
	if !credential.Flags.UserVerified || !credential.Flags.BackupEligible || !credential.Flags.BackupState {
		t.Fatalf("credential flags = %+v", credential.Flags)
	}
	if len(credential.Transport) != 2 || credential.Transport[0] != "internal" || credential.Transport[1] != "hybrid" {
		t.Fatalf("transports = %+v", credential.Transport)
	}
	wantAAGUID, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	if !bytes.Equal(credential.Authenticator.AAGUID, wantAAGUID) {
		t.Fatalf("AAGUID = %x", credential.Authenticator.AAGUID)
	}

	var user struct {
		WebAuthnID     []byte   `bson:"webauthnId"`
		ImportedIDs    []string `bson:"legacyWebAuthnImported"`
		Authenticators []bson.M `bson:"authenticators"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if string(user.WebAuthnID) != "7" || len(user.Authenticators) != 1 || len(user.ImportedIDs) != 1 || user.ImportedIDs[0] != id {
		t.Fatalf("legacy user = %+v", user)
	}

	rerun, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{
		Sources: []string{"contest2025"}, DryRun: false,
	})
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if !rerun.OK || rerun.Imported != 0 || rerun.PlannedImports != 0 || rerun.Existing != 1 || rerun.UsersUpdated != 0 {
		t.Fatalf("rerun report = %+v", rerun)
	}
	if count, err := db.Collection("webauthn_credentials").CountDocuments(ctx, bson.M{}); err != nil || count != 1 {
		t.Fatalf("rerun credential count=%d err=%v", count, err)
	}

	if _, err := db.Collection("webauthn_credentials").DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		t.Fatal(err)
	}
	afterDelete, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{
		Sources: []string{"contest2025"}, DryRun: false,
	})
	if err != nil {
		t.Fatalf("post-delete rerun: %v", err)
	}
	if !afterDelete.OK || afterDelete.Imported != 0 || afterDelete.SkippedFormerly != 1 || afterDelete.Existing != 0 {
		t.Fatalf("post-delete report = %+v", afterDelete)
	}
	if count, err := db.Collection("webauthn_credentials").CountDocuments(ctx, bson.M{}); err != nil || count != 0 {
		t.Fatalf("deleted credential was resurrected: count=%d err=%v", count, err)
	}
}

func TestLegacyWebAuthnImportRejectsDuplicateCredentialBeforeWriting(t *testing.T) {
	ctx, db := newWebAuthnMigrationDB(t)
	credentialID := []byte{9, 8, 7}
	authenticator := bson.M{
		"name": "Duplicate", "fmt": "none", "counter": int64(0),
		"credentialID":        bson.Binary{Subtype: 0, Data: credentialID},
		"credentialPublicKey": bson.Binary{Subtype: 0, Data: []byte{1, 2, 3}},
	}
	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(7), "uname": "alice", "authenticators": []any{authenticator}},
		bson.M{"_id": int64(8), "uname": "bob", "authenticators": []any{authenticator}},
	}); err != nil {
		t.Fatal(err)
	}
	report, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{DryRun: false})
	if err == nil || report == nil || report.OK || len(report.Conflicts) == 0 {
		t.Fatalf("conflict result = %+v, err=%v", report, err)
	}
	if count, err := db.Collection("webauthn_credentials").CountDocuments(ctx, bson.M{}); err != nil || count != 0 {
		t.Fatalf("conflict wrote credentials: count=%d err=%v", count, err)
	}
	var user struct {
		WebAuthnID []byte `bson:"webauthnId"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if len(user.WebAuthnID) != 0 {
		t.Fatalf("conflict wrote WebAuthn handle: %x", user.WebAuthnID)
	}
}

func TestLegacyWebAuthnImportRejectsChangingNativeHandle(t *testing.T) {
	ctx, db := newWebAuthnMigrationDB(t)
	nativeHandle := []byte("native-random-handle")
	_, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "webauthnId": nativeHandle,
		"authenticators": []any{bson.M{
			"credentialID":        bson.Binary{Subtype: 0, Data: []byte{1, 2, 3}},
			"credentialPublicKey": bson.Binary{Subtype: 0, Data: []byte{4, 5, 6}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("webauthn_credentials").InsertOne(ctx, bson.M{
		"_id": "native", "uid": int64(7), "name": "Native", "data": "{}",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{DryRun: false})
	if err == nil || report == nil || report.OK || len(report.Conflicts) == 0 {
		t.Fatalf("handle conflict result = %+v, err=%v", report, err)
	}
	var user struct {
		WebAuthnID []byte `bson:"webauthnId"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(user.WebAuthnID, nativeHandle) {
		t.Fatalf("native handle changed: %x", user.WebAuthnID)
	}
	if count, err := db.Collection("webauthn_credentials").CountDocuments(ctx, bson.M{}); err != nil || count != 1 {
		t.Fatalf("handle conflict wrote credentials: count=%d err=%v", count, err)
	}
}

func TestLegacyWebAuthnImportRepairsPartiallyImportedHandle(t *testing.T) {
	ctx, db := newWebAuthnMigrationDB(t)
	credentialID := []byte{7, 7, 7}
	id := base64.RawURLEncoding.EncodeToString(credentialID)
	_, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice",
		"authenticators": []any{bson.M{
			"credentialID":        bson.Binary{Subtype: 0, Data: credentialID},
			"credentialPublicKey": bson.Binary{Subtype: 0, Data: []byte{1, 2, 3}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("webauthn_credentials").InsertOne(ctx, bson.M{
		"_id": id, "uid": int64(7), "name": "Partial", "data": "{}",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{DryRun: false})
	if err != nil || !report.OK || report.Existing != 1 || report.UsersUpdated != 1 {
		t.Fatalf("partial import report = %+v, err=%v", report, err)
	}
	var user struct {
		WebAuthnID []byte `bson:"webauthnId"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if string(user.WebAuthnID) != "7" {
		t.Fatalf("repaired handle = %x", user.WebAuthnID)
	}
}

func TestLegacyWebAuthnImportLeavesNativeOnlyUsersUntouched(t *testing.T) {
	ctx, db := newWebAuthnMigrationDB(t)
	nativeHandle := []byte("native-random-handle")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(8), "uname": "native-only", "webauthnId": nativeHandle,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("webauthn_credentials").InsertOne(ctx, bson.M{
		"_id": "native-only", "uid": int64(8), "name": "Native", "data": "{}",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{DryRun: false})
	if err != nil || !report.OK || report.Users != 0 || report.Imported != 0 {
		t.Fatalf("native-only report = %+v, err=%v", report, err)
	}
	var user struct {
		WebAuthnID []byte `bson:"webauthnId"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(8)}).Decode(&user); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(user.WebAuthnID, nativeHandle) {
		t.Fatalf("native-only handle changed: %x", user.WebAuthnID)
	}
}

func TestWebAuthnTimeValueSupportsSecondsAndRFC3339(t *testing.T) {
	// Keep the parser behavior pinned through the public migration path. These
	// cases mirror legacy numeric Date.now() values and stringified dates.
	ctx, db := newWebAuthnMigrationDB(t)
	for i, regat := range []any{int64(1700000000), "2023-11-14T22:13:20Z"} {
		uid := int64(10 + i)
		_, err := db.Collection("user").InsertOne(ctx, bson.M{
			"_id": uid, "uname": fmt.Sprintf("user%d", uid),
			"authenticators": []any{bson.M{
				"credentialID":        bson.Binary{Subtype: 0, Data: []byte{byte(uid)}},
				"credentialPublicKey": bson.Binary{Subtype: 0, Data: []byte{1}},
				"regat":               regat,
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	report, err := migration.RunLegacyWebAuthn(ctx, db, migration.WebAuthnOptions{DryRun: false})
	if err != nil || !report.OK || report.Imported != 2 {
		t.Fatalf("time import report = %+v, err=%v", report, err)
	}
}
