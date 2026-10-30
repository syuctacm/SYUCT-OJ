package api_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestLostPasswordReset(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_lostpass_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("old-secret", "salt")
	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro",
			"priv": user.DefaultPriv},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("webauthn_credentials").InsertOne(ctx, bson.M{
		"_id": "credential-1", "uid": int64(7), "name": "laptop", "data": "{}",
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions, err := auth.NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatalf("session manager: %v", err)
	}
	authHandler := auth.NewHandler(userRepo, sessions)
	lostPassHandler := handler.NewLostPassHandler(userRepo, sessions, nil)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	lostPassHandler.RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	oldCookies := loginAssetUser(t, server.URL, "alice", "old-secret")
	if _, err := db.Collection("user").UpdateOne(ctx, bson.M{"_id": int64(7)}, bson.M{"$set": bson.M{"tfa": "JBSWY3DPEHPK3PXP"}}); err != nil {
		t.Fatal(err)
	}
	status, body := postJSON(t, server.URL+"/api/v1/auth/lostpass", map[string]string{"mail": "alice@example.com"}, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"sent":false`)) || !bytes.Contains(body, []byte(`"code":"`)) {
		t.Fatalf("request reset = %d: %s", status, body)
	}
	var request struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &request); err != nil || request.Data.Code == "" {
		t.Fatalf("reset request payload = %s: %v", body, err)
	}

	resp, err := http.Get(server.URL + "/api/v1/auth/lostpass/" + request.Data.Code)
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Data struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || info.Data.Username != "alice" {
		t.Fatalf("reset info = %d: %+v", resp.StatusCode, info.Data)
	}

	status, body = postJSON(t, server.URL+"/api/v1/auth/lostpass/"+request.Data.Code, map[string]string{
		"password": "new-secret", "verifyPassword": "new-secret",
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("reset password = %d: %s", status, body)
	}

	status, _ = postJSON(t, server.URL+"/api/v1/auth/lostpass/"+request.Data.Code, map[string]string{
		"password": "another-secret", "verifyPassword": "another-secret",
	}, nil)
	if status != http.StatusNotFound {
		t.Fatalf("reused token = %d, want 404", status)
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/me", nil)
	for _, cookie := range oldCookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old session status = %d, want 401", resp.StatusCode)
	}

	if status, _ := postJSON(t, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "old-secret",
	}, nil); status != http.StatusUnauthorized {
		t.Fatalf("old password login = %d", status)
	}
	if status, _ := postJSON(t, server.URL+"/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "new-secret",
	}, nil); status != http.StatusOK {
		t.Fatalf("new password login = %d", status)
	}

	var stored struct {
		TFA string `bson:"tfa"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.TFA != "" {
		t.Fatalf("tfa was not cleared: %q", stored.TFA)
	}
	if count, err := db.Collection("webauthn_credentials").CountDocuments(ctx, bson.M{"uid": int64(7)}); err != nil || count != 0 {
		t.Fatalf("webauthn credentials count=%d err=%v", count, err)
	}
	if count, err := db.Collection("token").CountDocuments(ctx, bson.M{"tokenType": user.LostPassTokenType}); err != nil || count != 0 {
		t.Fatalf("lostpass token count=%d err=%v", count, err)
	}

	// With SMTP the response must not contain the token, and the mail must be
	// sent to the registered address.
	fake := &fakeMailer{}
	smtpRouter := gin.New()
	authHandler.RegisterRoutes(smtpRouter.Group("/"))
	handler.NewLostPassHandler(userRepo, sessions, fake).RegisterRoutes(smtpRouter.Group("/"))
	smtpServer := httptest.NewServer(smtpRouter)
	defer smtpServer.Close()
	status, body = postJSON(t, smtpServer.URL+"/api/v1/auth/lostpass", map[string]string{"mail": "bob@example.com"}, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"sent":true`)) || bytes.Contains(body, []byte(`"code":"`)) {
		t.Fatalf("smtp reset request = %d: %s", status, body)
	}
	if fake.to != "bob@example.com" || fake.subject != "重置密码" || fake.body == "" {
		t.Fatalf("mailer = %+v", fake)
	}
}
