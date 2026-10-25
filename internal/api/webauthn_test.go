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
	"github.com/syuctacm/syuct-oj/internal/webauthn"
)

func TestWebAuthnHandlerLifecycle(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_webauthn_api_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()
	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro",
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	service, err := webauthn.New(ctx, db, userRepo, webauthn.Config{
		RPID: "localhost", RPDisplayName: "SYUCT OJ", RPOrigins: []string{"http://localhost:5174"},
	})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	auth.NewHandler(userRepo, sessions).RegisterRoutes(router.Group("/"))
	handler.NewWebAuthnHandler(service, userRepo, sessions).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	if status, _ := postJSON(t, server.URL+"/api/v1/user/security/webauthn/register/begin", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous begin registration = %d", status)
	}
	if status, _ := getJSON(t, server.URL+"/api/v1/user/security/webauthn", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous passkey list = %d", status)
	}
	if status, _ := postJSON(t, server.URL+"/api/v1/auth/webauthn/login/begin", map[string]string{"username": "alice"}, nil); status != http.StatusNotFound {
		t.Fatalf("begin login without credential = %d", status)
	}

	cookies := loginAssetUser(t, server.URL, "alice", "secret")
	status, body := getJSON(t, server.URL+"/api/v1/user/security/webauthn", cookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"items":[]`)) {
		t.Fatalf("empty passkey list = %d: %s", status, body)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/webauthn/register/begin", nil, cookies)
	if status != http.StatusOK {
		t.Fatalf("begin registration = %d: %s", status, body)
	}
	var began struct {
		Token   string `json:"token"`
		Options struct {
			PublicKey struct {
				Challenge string `json:"challenge"`
			} `json:"publicKey"`
		} `json:"options"`
	}
	if err := json.Unmarshal(body, &began); err != nil {
		t.Fatalf("begin payload = %s: %v", body, err)
	}
	if began.Token == "" || began.Options.PublicKey.Challenge == "" {
		t.Fatalf("begin payload = %s", body)
	}
	var stored struct {
		WebAuthnID []byte `bson:"webauthnId"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.WebAuthnID) != 32 {
		t.Fatalf("WebAuthn handle length = %d", len(stored.WebAuthnID))
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/webauthn/register/finish", map[string]any{
		"token": began.Token, "name": "Test Passkey", "credential": map[string]any{},
	}, cookies)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid registration finish = %d: %s", status, body)
	}
	status, body = postJSON(t, server.URL+"/api/v1/auth/webauthn/login/finish", map[string]any{
		"token": "missing", "credential": map[string]any{},
	}, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("invalid login finish = %d: %s", status, body)
	}
}
