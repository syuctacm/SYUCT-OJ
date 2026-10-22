package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestUserRegistrationIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_registration_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	call := func(method, path, body string, cookies []*http.Cookie) (int, []byte, []*http.Cookie) {
		var reader io.Reader
		if body != "" {
			reader = bytes.NewBufferString(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, reader)
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data, resp.Cookies()
	}

	if status, _, _ := call(http.MethodPost, "/api/v1/auth/register", `{"mail":"not-an-email"}`, nil); status != http.StatusBadRequest {
		t.Fatalf("invalid email = %d", status)
	}

	status, body, _ := call(http.MethodPost, "/api/v1/auth/register", `{"mail":"new@example.com"}`, nil)
	if status != http.StatusOK {
		t.Fatalf("register request = %d: %s", status, body)
	}
	var requested struct {
		Data struct {
			Code string `json:"code"`
			Mail string `json:"mail"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &requested); err != nil || requested.Data.Code == "" {
		t.Fatalf("register payload = %s (%v)", body, err)
	}
	code := requested.Data.Code

	status, body, _ = call(http.MethodGet, "/api/v1/auth/register/"+code, "", nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte("new@example.com")) {
		t.Fatalf("register info = %d: %s", status, body)
	}

	if status, _, _ = call(http.MethodPost, "/api/v1/auth/register/"+code,
		`{"uname":"newbie","password":"123","verifyPassword":"123"}`, nil); status != http.StatusBadRequest {
		t.Fatalf("short password = %d", status)
	}
	if status, _, _ = call(http.MethodPost, "/api/v1/auth/register/"+code,
		`{"uname":"newbie","password":"secret123","verifyPassword":"different"}`, nil); status != http.StatusBadRequest {
		t.Fatalf("password mismatch = %d", status)
	}
	status, body, cookies := call(http.MethodPost, "/api/v1/auth/register/"+code,
		`{"uname":"newbie","password":"secret123","verifyPassword":"secret123"}`, nil)
	if status != http.StatusCreated {
		t.Fatalf("register complete = %d: %s", status, body)
	}
	if len(cookies) == 0 || cookies[0].Name != "sid" {
		t.Fatalf("session cookie missing: %#v", cookies)
	}
	if status, body, _ := call(http.MethodGet, "/api/v1/auth/me", "", cookies); status != http.StatusOK || !bytes.Contains(body, []byte("newbie")) {
		t.Fatalf("me after register = %d: %s", status, body)
	}

	var account struct {
		ID     int64  `bson:"_id"`
		Mail   string `bson:"mail"`
		Uname  string `bson:"uname"`
		Priv   int64  `bson:"priv"`
		Salt   string `bson:"salt"`
		Hash   string `bson:"hash"`
		Avatar string `bson:"avatar"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"uname": "newbie"}).Decode(&account); err != nil {
		t.Fatal(err)
	}
	if account.Priv != user.DefaultPriv || account.Avatar != "gravatar:new@example.com" {
		t.Fatalf("account defaults = %#v", account)
	}
	if user.VerifyPassword(&user.User{Salt: account.Salt, Hash: account.Hash, HashType: "hydro"}, "secret123") != nil {
		t.Fatal("stored password does not verify")
	}
	if count, _ := db.Collection("domain.user").CountDocuments(ctx, bson.M{"domainId": "system", "uid": account.ID, "join": true}); count != 1 {
		t.Fatalf("domain.user join row missing")
	}
	if status, _, _ := call(http.MethodGet, "/api/v1/auth/register/"+code, "", nil); status != http.StatusNotFound {
		t.Fatalf("token was not consumed = %d", status)
	}

	if _, _, loginCookies := call(http.MethodPost, "/api/v1/auth/login",
		`{"username":"newbie","password":"secret123"}`, nil); len(loginCookies) == 0 {
		t.Fatal("login with the new account failed")
	}
	if status, _, _ := call(http.MethodPost, "/api/v1/auth/register", `{"mail":"new@example.com"}`, nil); status != http.StatusConflict {
		t.Fatalf("duplicate email = %d", status)
	}

	status, body, _ = call(http.MethodPost, "/api/v1/auth/register", `{"mail":"other@example.com"}`, nil)
	if status != http.StatusOK {
		t.Fatalf("second register request = %d: %s", status, body)
	}
	var second struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &second)
	if status, _, _ := call(http.MethodPost, "/api/v1/auth/register/"+second.Data.Code,
		`{"uname":"newbie","password":"secret123","verifyPassword":"secret123"}`, nil); status != http.StatusConflict {
		t.Fatalf("duplicate username = %d", status)
	}
}
