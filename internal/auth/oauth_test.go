package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestOAuthGitHubLoginAndBinding(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_oauth_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("code") != "good-code" {
				http.Error(w, `{"error":"bad_code"}`, http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"bearer"}`))
		case "/user":
			if r.Header.Get("Authorization") != "token access-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"id":12345,"login":"octocat","name":"Octo Cat","email":"","avatar_url":"https://example.com/avatar.png","bio":"hi"}`))
		case "/emails":
			_, _ = w.Write([]byte(`[{"email":"octocat@example.com","primary":true,"verified":true}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	cfg := config.Config{
		OAuthCallbackBaseURL:    "http://oj.test",
		OAuthGitHubClientID:     "client-id",
		OAuthGitHubClientSecret: "client-secret",
		OAuthGitHubAuthorizeURL: provider.URL + "/authorize",
		OAuthGitHubTokenURL:     provider.URL + "/token",
		OAuthGitHubUserURL:      provider.URL + "/user",
		OAuthGitHubEmailsURL:    provider.URL + "/emails",
		OAuthGitHubScope:        "read:user user:email",
		OAuthGitHubCanRegister:  true,
		OAuthGitHubName:         "GitHub",
	}
	users := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	handler := auth.NewOAuthHandler(db, users, sessions, cfg)
	if _, err := db.Collection("system").InsertOne(ctx, bson.M{"_id": "server.url", "value": "https://oj.example.com/oj"}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler.RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	clientHTTP := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	loginReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/oauth/github/login?redirect=%2Fp", nil)
	loginResp, err := clientHTTP.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusFound {
		t.Fatalf("oauth login status = %d", loginResp.StatusCode)
	}
	location, err := url.Parse(loginResp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Path != "/authorize" || location.Query().Get("state") == "" || location.Query().Get("client_id") != "client-id" {
		t.Fatalf("oauth authorize location = %s", location)
	}
	if redirectURI := location.Query().Get("redirect_uri"); redirectURI != "https://oj.example.com/oj/api/v1/auth/oauth/github/callback" {
		t.Fatalf("oauth redirect_uri = %q", redirectURI)
	}
	stateCookie := loginResp.Cookies()[0]
	state := location.Query().Get("state")
	callbackReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/oauth/github/callback?code=good-code&state="+url.QueryEscape(state), nil)
	callbackReq.AddCookie(stateCookie)
	callbackResp, err := clientHTTP.Do(callbackReq)
	if err != nil {
		t.Fatal(err)
	}
	defer callbackResp.Body.Close()
	if callbackResp.StatusCode != http.StatusFound || callbackResp.Header.Get("Location") != "/p" {
		t.Fatalf("oauth callback status=%d location=%q", callbackResp.StatusCode, callbackResp.Header.Get("Location"))
	}
	var cookies []*http.Cookie
	cookies = append(cookies, callbackResp.Cookies()...)
	sessionCookie := findCookie(cookies, "sid")
	if sessionCookie == nil {
		t.Fatal("oauth callback did not issue session cookie")
	}
	session, ok := sessions.GetContext(ctx, sessionCookie.Value)
	if !ok || session.Kind != "user" {
		t.Fatal("oauth session was not issued")
	}
	account, err := users.FindByID(ctx, session.UserID)
	if err != nil || account.Mail != "octocat@example.com" || account.Uname != "octocat" || account.Avatar != "https://example.com/avatar.png" {
		t.Fatalf("oauth account = %+v err=%v", account, err)
	}
	var link auth.OAuthLink
	if err := db.Collection("oauth").FindOne(ctx, bson.M{"platform": "github", "id": "12345"}).Decode(&link); err != nil || link.UID != account.ID {
		t.Fatalf("oauth link = %+v err=%v", link, err)
	}

	// A second user can bind the same provider after the first identity is
	// explicitly unlinked.
	second, err := users.Create(ctx, user.RegisterInput{Mail: "second@example.com", Username: "second", Password: "secret123", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	bindToken, err := sessions.IssueForUserMeta(ctx, second.ID, "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("oauth").DeleteMany(ctx, bson.M{"uid": account.ID}); err != nil {
		t.Fatal(err)
	}
	bindReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/oauth/github/login?bind=1", nil)
	bindReq.AddCookie(&http.Cookie{Name: "sid", Value: bindToken})
	bindResp, err := clientHTTP.Do(bindReq)
	if err != nil {
		t.Fatal(err)
	}
	defer bindResp.Body.Close()
	if bindResp.StatusCode != http.StatusFound {
		t.Fatalf("oauth bind login status = %d", bindResp.StatusCode)
	}
	bindStateCookie := findCookie(bindResp.Cookies(), "oauth_state")
	bindLocation, _ := url.Parse(bindResp.Header.Get("Location"))
	bindState := bindLocation.Query().Get("state")
	bindCallback, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/oauth/github/callback?code=good-code&state="+url.QueryEscape(bindState), nil)
	bindCallback.AddCookie(bindStateCookie)
	bindCallback.AddCookie(&http.Cookie{Name: "sid", Value: bindToken})
	bindCallbackResp, err := clientHTTP.Do(bindCallback)
	if err != nil {
		t.Fatal(err)
	}
	defer bindCallbackResp.Body.Close()
	if bindCallbackResp.StatusCode != http.StatusFound || bindCallbackResp.Header.Get("Location") != "/home/security" {
		t.Fatalf("oauth bind callback status=%d location=%q", bindCallbackResp.StatusCode, bindCallbackResp.Header.Get("Location"))
	}
	if err := db.Collection("oauth").FindOne(ctx, bson.M{"platform": "github", "id": "12345"}).Decode(&link); err != nil || link.UID != second.ID {
		t.Fatalf("rebound oauth link = %+v err=%v", link, err)
	}

	// State replay must be rejected after the one-time state document is consumed.
	replay, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/oauth/github/callback?code=good-code&state="+url.QueryEscape(bindState), nil)
	replay.AddCookie(bindStateCookie)
	replay.AddCookie(&http.Cookie{Name: "sid", Value: bindToken})
	replayResp, err := clientHTTP.Do(replay)
	if err != nil {
		t.Fatal(err)
	}
	defer replayResp.Body.Close()
	if replayResp.StatusCode != http.StatusFound || !strings.Contains(replayResp.Header.Get("Location"), "oauth_error=") {
		t.Fatalf("oauth state replay status=%d location=%q", replayResp.StatusCode, replayResp.Header.Get("Location"))
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
