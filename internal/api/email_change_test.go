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
	"github.com/syuctacm/syuct-oj/internal/mail"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type fakeMailer struct {
	to      string
	subject string
	body    string
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.to, f.subject, f.body = to, subject, body
	return nil
}

func TestEmailChangeIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_email_change_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(1), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756)},
		bson.M{"_id": int64(2), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756)},
	}); err != nil {
		t.Fatal(err)
	}
	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	emailHandler := handler.NewUserEmailHandler(userRepo, sessions, nil)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	emailHandler.RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")
	bobCookies := loginAssetUser(t, server.URL, "bob", "secret")
	status, body := postJSON(t, server.URL+"/api/v1/user/security/email", map[string]string{"current": "wrong", "email": "new@example.com"}, aliceCookies)
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d: %s", status, body)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/email", map[string]string{"current": "secret", "email": "bad"}, aliceCookies)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid email = %d: %s", status, body)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/email", map[string]string{"current": "secret", "email": "bob@example.com"}, aliceCookies)
	if status != http.StatusConflict {
		t.Fatalf("existing email = %d: %s", status, body)
	}

	status, body = postJSON(t, server.URL+"/api/v1/user/security/email", map[string]string{"current": "secret", "email": "Alice.New@Example.com"}, aliceCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"sent":false`)) || !bytes.Contains(body, []byte(`"code":`)) {
		t.Fatalf("request = %d: %s", status, body)
	}
	var request struct {
		Data struct {
			Code string `json:"code"`
			Link string `json:"link"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("request payload = %s: %v", body, err)
	}
	if request.Data.Code == "" || request.Data.Link != "/home/changeMail/"+request.Data.Code {
		t.Fatalf("request data = %+v", request.Data)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/email/confirm", map[string]string{"code": request.Data.Code}, bobCookies)
	if status != http.StatusForbidden {
		t.Fatalf("other user confirm = %d: %s", status, body)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/email/confirm", map[string]string{"code": request.Data.Code}, aliceCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"email":"Alice.New@Example.com"`)) {
		t.Fatalf("confirm = %d: %s", status, body)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/email/confirm", map[string]string{"code": request.Data.Code}, aliceCookies)
	if status != http.StatusBadRequest {
		t.Fatalf("reused token = %d: %s", status, body)
	}
	var stored struct {
		Mail      string `bson:"mail"`
		MailLower string `bson:"mailLower"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(1)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Mail != "Alice.New@Example.com" || stored.MailLower != "alice.new@example.com" {
		t.Fatalf("stored mail = %+v", stored)
	}

	// The SMTP path must send and not expose a confirmation code.
	fake := &fakeMailer{}
	smtpHandler := handler.NewUserEmailHandler(userRepo, sessions, fake)
	smtpRouter := gin.New()
	authHandler.RegisterRoutes(smtpRouter.Group("/"))
	smtpHandler.RegisterRoutes(smtpRouter.Group("/"))
	smtpServer := httptest.NewServer(smtpRouter)
	defer smtpServer.Close()
	smtpCookies := loginAssetUser(t, smtpServer.URL, "alice", "secret")
	status, body = postJSON(t, smtpServer.URL+"/api/v1/user/security/email", map[string]string{"current": "secret", "email": "smtp@example.com"}, smtpCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"sent":true`)) || bytes.Contains(body, []byte(`"code":"`)) {
		t.Fatalf("smtp request = %d: %s", status, body)
	}
	if fake.to != "smtp@example.com" || fake.subject != "修改邮箱" || fake.body == "" {
		t.Fatalf("mailer = %+v", fake)
	}
}

var _ mail.Sender = (*fakeMailer)(nil)
