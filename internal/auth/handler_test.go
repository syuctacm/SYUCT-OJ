package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type fakeStore struct {
	u *user.User
}

func (f fakeStore) FindByLogin(context.Context, string) (*user.User, error) {
	return f.u, nil
}

func (f fakeStore) FindByID(context.Context, int64) (*user.User, error) {
	return f.u, nil
}

func TestAuthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := &user.User{
		ID: 7, Uname: "Alice", Mail: "alice@example.com",
		Salt: "salt", Hash: user.HashPassword("secret", "salt"), HashType: "hydro",
	}
	sessions := auth.NewSessionManager(time.Hour)
	handler := auth.NewHandler(fakeStore{u: u}, sessions)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/"))

	body, _ := json.Marshal(map[string]string{"username": "Alice", "password": "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", resp.Code, resp.Body.String())
	}
	cookies := resp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set cookie")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	for _, cookie := range cookies {
		meReq.AddCookie(cookie)
	}
	meResp := httptest.NewRecorder()
	router.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("me status = %d: %s", meResp.Code, meResp.Body.String())
	}
	var me struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(meResp.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.User.ID != 7 {
		t.Fatalf("me user id = %d", me.User.ID)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	for _, cookie := range cookies {
		logoutReq.AddCookie(cookie)
	}
	logoutResp := httptest.NewRecorder()
	router.ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResp.Code)
	}
	meReq = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	for _, cookie := range cookies {
		meReq.AddCookie(cookie)
	}
	meResp = httptest.NewRecorder()
	router.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d", meResp.Code)
	}
}
