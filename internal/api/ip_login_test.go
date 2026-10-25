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
	"github.com/syuctacm/syuct-oj/internal/api/middleware"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestIPAutoLoginIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_ip_login_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
			"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(4)},
	}); err != nil {
		t.Fatal(err)
	}
	userRepo := user.NewRepository(db)
	if err := userRepo.SetIPLogin(ctx, "10.1.2.3", 7); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)

	router := gin.New()
	router.Use(middleware.IPAutoLogin(true, userRepo, sessions))
	authHandler.RegisterRoutes(router.Group("/"))

	rec := serveIPRequest(router, http.MethodGet, "/api/v1/auth/me", "10.1.2.3:12345", nil)
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) == 0 {
		t.Fatalf("ip login status = %d cookies=%d", rec.Code, len(rec.Result().Cookies()))
	}
	cookies := rec.Result().Cookies()

	rec = serveIPRequest(router, http.MethodGet, "/api/v1/auth/me", "10.1.2.3:12345", cookies)
	var me struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != http.StatusOK || me.User.ID != 7 {
		t.Fatalf("authenticated ip session = %d id=%d", rec.Code, me.User.ID)
	}

	rec = serveIPRequest(router, http.MethodGet, "/api/v1/auth/me", "10.1.2.4:12345", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unmapped IP = %d", rec.Code)
	}

	rec = serveIPRequest(router, http.MethodPost, "/api/v1/auth/logout", "10.1.2.3:12345", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout under ip login = %d", rec.Code)
	}

	disabled := gin.New()
	disabled.Use(middleware.IPAutoLogin(false, userRepo, sessions))
	authHandler.RegisterRoutes(disabled.Group("/"))
	rec = serveIPRequest(disabled, http.MethodGet, "/api/v1/auth/me", "10.1.2.3:12345", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("disabled ip login = %d", rec.Code)
	}

	uids, messages, err := userRepo.ImportUsers(ctx, []user.ImportUser{{
		Email: "onsite@example.com", Username: "onsite", Password: "secret", IP: "10.2.3.4",
	}}, "127.0.0.1")
	if err != nil || len(uids) != 1 || len(messages) != 0 {
		t.Fatalf("onsite import = uids=%v messages=%v err=%v", uids, messages, err)
	}
	if uid, found, err := userRepo.IPLoginUID(ctx, "10.2.3.4"); err != nil || !found || uid != uids[0] {
		t.Fatalf("imported IP mapping = uid=%d found=%v err=%v", uid, found, err)
	}

	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	adminRouter := gin.New()
	authHandler.RegisterRoutes(adminRouter.Group("/"))
	adminHandler.RegisterRoutes(adminRouter.Group("/"))
	adminServer := httptest.NewServer(adminRouter)
	defer adminServer.Close()
	adminCookies := loginAssetUser(t, adminServer.URL, "admin", "secret")
	requestBody, _ := json.Marshal(map[string]int64{"uid": 7})
	status, body := rawRequest(t, http.MethodPut, adminServer.URL+"/api/v1/admin/ip-login/10.9.9.9", requestBody, adminCookies)
	if status != http.StatusOK {
		t.Fatalf("set ip login = %d: %s", status, body)
	}
	status, body = getJSON(t, adminServer.URL+"/api/v1/admin/ip-login", adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"ip":"10.9.9.9"`)) || !bytes.Contains(body, []byte(`"username":"alice"`)) {
		t.Fatalf("list ip login = %d: %s", status, body)
	}
	req, _ := http.NewRequest(http.MethodDelete, adminServer.URL+"/api/v1/admin/ip-login/10.9.9.9", nil)
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete ip login = %d", resp.StatusCode)
	}
}

func serveIPRequest(router http.Handler, method, path, remote string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remote
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
