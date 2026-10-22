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
	"github.com/syuctacm/syuct-oj/internal/totp"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestTOTPIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_tfa_test_%d", time.Now().UnixNano())
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
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756),
	}); err != nil {
		t.Fatal(err)
	}
	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	userHandler := handler.NewUserHandler(userRepo, sessions, nil)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	userHandler.RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	cookies := loginAssetUser(t, server.URL, "alice", "secret")
	status, body := postJSON(t, server.URL+"/api/v1/user/security/tfa/setup", nil, cookies)
	if status != http.StatusOK {
		t.Fatalf("setup = %d: %s", status, body)
	}
	var setup struct {
		Data struct {
			Secret string `json:"secret"`
			URI    string `json:"uri"`
			QR     string `json:"qr"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &setup); err != nil {
		t.Fatalf("setup payload = %s: %v", body, err)
	}
	if len(setup.Data.Secret) != totp.SecretLength || setup.Data.URI == "" || !bytes.HasPrefix([]byte(setup.Data.QR), []byte("data:image/png;base64,")) {
		t.Fatalf("setup data = %+v", setup.Data)
	}
	code, err := totp.Code(setup.Data.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/tfa/enable", map[string]string{
		"secret": setup.Data.Secret, "code": code,
	}, cookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"tfa":true`)) {
		t.Fatalf("enable = %d: %s", status, body)
	}
	status, body = getJSON(t, server.URL+"/api/v1/auth/me", cookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"tfa":true`)) {
		t.Fatalf("me after enable = %d: %s", status, body)
	}

	loginBody, _ := json.Marshal(map[string]string{"username": "alice", "password": "secret"})
	status, body = rawRequest(t, http.MethodPost, server.URL+"/api/v1/auth/login", loginBody, nil)
	if status != http.StatusUnauthorized || !bytes.Contains(body, []byte("2FA required")) {
		t.Fatalf("login without tfa = %d: %s", status, body)
	}
	loginBody, _ = json.Marshal(map[string]string{"username": "alice", "password": "secret", "tfa": "000000"})
	status, body = rawRequest(t, http.MethodPost, server.URL+"/api/v1/auth/login", loginBody, nil)
	if status != http.StatusUnauthorized || !bytes.Contains(body, []byte("invalid 2FA code")) {
		t.Fatalf("login with bad tfa = %d: %s", status, body)
	}
	code, err = totp.Code(setup.Data.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	loginBody, _ = json.Marshal(map[string]string{"username": "alice", "password": "secret", "tfa": code})
	status, body = rawRequest(t, http.MethodPost, server.URL+"/api/v1/auth/login", loginBody, nil)
	if status != http.StatusOK {
		t.Fatalf("login with tfa = %d: %s", status, body)
	}

	status, body = postJSON(t, server.URL+"/api/v1/user/security/tfa/disable", map[string]string{"current": "wrong"}, cookies)
	if status != http.StatusUnauthorized {
		t.Fatalf("disable wrong password = %d: %s", status, body)
	}
	status, body = postJSON(t, server.URL+"/api/v1/user/security/tfa/disable", map[string]string{"current": "secret"}, cookies)
	if status != http.StatusNoContent {
		t.Fatalf("disable = %d: %s", status, body)
	}
	loginBody, _ = json.Marshal(map[string]string{"username": "alice", "password": "secret"})
	status, body = rawRequest(t, http.MethodPost, server.URL+"/api/v1/auth/login", loginBody, nil)
	if status != http.StatusOK {
		t.Fatalf("login after disable = %d: %s", status, body)
	}
}

func postJSON(t *testing.T, url string, value any, cookies []*http.Cookie) (int, []byte) {
	t.Helper()
	var body []byte
	if value != nil {
		var err error
		body, err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	return rawRequest(t, http.MethodPost, url, body, cookies)
}

func getJSON(t *testing.T, url string, cookies []*http.Cookie) (int, []byte) {
	t.Helper()
	return rawRequest(t, http.MethodGet, url, nil, cookies)
}

func rawRequest(t *testing.T, method, url string, body []byte, cookies []*http.Cookie) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
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
