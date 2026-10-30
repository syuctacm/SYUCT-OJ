package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestAccountDeletionGraceAndPurge(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_account_delete_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
		"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("domain.user").InsertOne(ctx, bson.M{"domainId": "system", "uid": int64(7), "join": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("message").InsertOne(ctx, bson.M{"from": int64(7), "to": []int64{8}, "content": "hi", "flag": 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("webauthn_credentials").InsertOne(ctx, bson.M{"_id": "cred", "uid": int64(7)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(7), "pid": int64(1), "status": 1}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions, err := auth.NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatalf("session manager: %v", err)
	}
	authHandler := auth.NewHandler(userRepo, sessions)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	handler.NewUserDeleteHandler(userRepo, sessions).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()
	cookies := loginAssetUser(t, server.URL, "alice", "secret")

	call := func(method, path, body string) (int, []byte) {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data := new(bytes.Buffer)
		_, _ = data.ReadFrom(resp.Body)
		return resp.StatusCode, data.Bytes()
	}

	if status, _ := call(http.MethodPost, "/api/v1/user/delete", `{"password":"wrong"}`); status != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d", status)
	}
	status, body := call(http.MethodPost, "/api/v1/user/delete", `{"password":"secret"}`)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"deleteAt"`)) {
		t.Fatalf("request delete = %d: %s", status, body)
	}
	if status, _ := call(http.MethodPost, "/api/v1/user/delete/cancel", ""); status != http.StatusOK {
		t.Fatalf("cancel delete = %d", status)
	}
	if status, body := call(http.MethodGet, "/api/v1/user/delete", ""); status != http.StatusOK || !bytes.Contains(body, []byte(`"deleteAt":null`)) {
		t.Fatalf("status after cancel = %d: %s", status, body)
	}
	if status, _ := call(http.MethodPost, "/api/v1/user/delete", `{"password":"secret"}`); status != http.StatusOK {
		t.Fatalf("second request = %d", status)
	}
	past := time.Now().Add(-time.Hour)
	if _, err := db.Collection("user").UpdateOne(ctx, bson.M{"_id": int64(7)}, bson.M{"$set": bson.M{"deleteAt": past}}); err != nil {
		t.Fatal(err)
	}
	count, err := user.PurgeExpiredUsers(ctx, db, sessions)
	if err != nil || count != 1 {
		t.Fatalf("purge count=%d err=%v", count, err)
	}
	var stored struct {
		Uname  string     `bson:"uname"`
		Mail   string     `bson:"mail"`
		Banned bool       `bson:"banned"`
		Delete *time.Time `bson:"deleteAt"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Uname != "deleted-7" || stored.Mail != "" || !stored.Banned || stored.Delete != nil {
		t.Fatalf("anonymized user = %+v", stored)
	}
	for _, check := range []struct {
		name   string
		filter bson.M
	}{
		{name: "domain.user", filter: bson.M{"uid": int64(7)}},
		{name: "webauthn_credentials", filter: bson.M{"uid": int64(7)}},
		{name: "message", filter: bson.M{"$or": []bson.M{{"from": int64(7)}, {"to": int64(7)}}}},
	} {
		n, err := db.Collection(check.name).CountDocuments(ctx, check.filter)
		if err != nil || n != 0 {
			t.Fatalf("%s count=%d err=%v", check.name, n, err)
		}
	}
	if n, err := db.Collection("record").CountDocuments(ctx, bson.M{"uid": int64(7)}); err != nil || n != 1 {
		t.Fatalf("record count=%d err=%v", n, err)
	}
	status, body = postJSON(t, server.URL+"/api/v1/auth/login", map[string]string{"username": "alice", "password": "secret"}, nil)
	if status != http.StatusForbidden && status != http.StatusUnauthorized {
		t.Fatalf("deleted login = %d: %s", status, body)
	}
	// The old session must also be invalid.
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/me", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("deleted session = %d", resp.StatusCode)
	}
}
