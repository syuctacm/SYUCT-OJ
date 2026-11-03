package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestPublicSettingsIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_settings_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com", "mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	adminHandler.SetSessionTTLUpdater(func(saved, unsaved time.Duration) {
		sessions.SetTTLs(saved, unsaved)
	})
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, nil, userRepo, adminHandler, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	type settingsPayload struct {
		Data struct {
			Bulletin             string `json:"bulletin"`
			SiteName             string `json:"siteName"`
			NavLogo              string `json:"navLogo"`
			FooterExtraHTML      string `json:"footerExtraHtml"`
			EnableScratchpad     bool   `json:"enableScratchpad"`
			GravatarURL          string `json:"gravatarUrl"`
			HomepageYAML         string `json:"homepageYaml"`
			AboutMarkdown        string `json:"aboutMarkdown"`
			LanguageYAML         string `json:"languageYaml"`
			PaginationProblem    int    `json:"paginationProblem"`
			PaginationContest    int    `json:"paginationContest"`
			PaginationDiscussion int    `json:"paginationDiscussion"`
			PaginationRecord     int    `json:"paginationRecord"`
			PaginationRanking    int    `json:"paginationRanking"`
			PaginationSolution   int    `json:"paginationSolution"`
			PaginationTraining   int    `json:"paginationTraining"`
			LimitUserFiles       int    `json:"limitUserFiles"`
			LimitUserFileSize    int64  `json:"limitUserFileSize"`
			LimitContestFiles    int    `json:"limitContestFiles"`
			LimitContestFileSize int64  `json:"limitContestFileSize"`
			LimitProblemFiles    int    `json:"limitProblemFiles"`
			LimitProblemFileSize int64  `json:"limitProblemFileSize"`
			LimitCodeLength      int64  `json:"limitCodeLength"`
			LimitSubmission      int    `json:"limitSubmission"`
			LimitSubmissionUser  int    `json:"limitSubmissionUser"`
			LimitPretest         int    `json:"limitPretest"`
			LimitByUser          bool   `json:"limitByUser"`
			SessionSavedExpire   int64  `json:"sessionSavedExpireSeconds"`
			SessionUnsavedExpire int64  `json:"sessionUnsavedExpireSeconds"`
		} `json:"data"`
	}
	get := func(path string, cookies []*http.Cookie) (int, settingsPayload) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var payload settingsPayload
		if resp.StatusCode == http.StatusOK {
			if err := decodeJSON(resp.Body, &payload); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, payload
	}
	put := func(body string, cookies []*http.Cookie) int {
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/admin/settings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	status, payload := get("/api/v1/settings", nil)
	if status != http.StatusOK || payload.Data.Bulletin != "" || payload.Data.SiteName != handler.DefaultSiteName ||
		payload.Data.NavLogo != handler.DefaultNavLogo || !payload.Data.EnableScratchpad ||
		payload.Data.GravatarURL != handler.DefaultGravatarURL || payload.Data.HomepageYAML == "" || payload.Data.LanguageYAML == "" ||
		payload.Data.PaginationProblem != 100 || payload.Data.PaginationContest != 20 ||
		payload.Data.PaginationDiscussion != 50 || payload.Data.PaginationRecord != 100 ||
		payload.Data.PaginationRanking != 100 || payload.Data.PaginationSolution != 20 ||
		payload.Data.PaginationTraining != 10 || payload.Data.LimitUserFiles != 100 ||
		payload.Data.LimitUserFileSize != 128<<20 || payload.Data.LimitContestFiles != 100 ||
		payload.Data.LimitContestFileSize != 128<<20 || payload.Data.LimitProblemFiles != 100 ||
		payload.Data.LimitProblemFileSize != 256<<20 || payload.Data.LimitCodeLength != 128<<10 ||
		payload.Data.LimitSubmission != 60 || payload.Data.LimitSubmissionUser != 15 ||
		payload.Data.LimitPretest != 60 || payload.Data.LimitByUser ||
		payload.Data.SessionSavedExpire != 30*24*60*60 || payload.Data.SessionUnsavedExpire != 3*60*60 {
		t.Fatalf("initial public settings = %d %+v", status, payload.Data)
	}

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")

	if status := put(`{"bulletin":"# 公告"}`, aliceCookies); status != http.StatusForbidden {
		t.Fatalf("non-admin update = %d", status)
	}
	if status := put(`{"bulletin":"# 公告"}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous update = %d", status)
	}
	if status := put(`{"bulletin":"`+strings.Repeat("x", 20001)+`"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("oversized bulletin = %d", status)
	}
	if status := put(`{"homepageYaml":"not: [valid"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid homepage yaml = %d", status)
	}
	if status := put(`{"homepageYaml":"- width: 13\n  contest: 1\n"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid homepage width = %d", status)
	}
	if status := put(`{"homepageYaml":"- width: 12\n  contest: {limit: 1}\n"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid homepage section = %d", status)
	}
	if status := put(`{"siteName":""}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("empty site name = %d", status)
	}
	if status := put(`{"navLogo":"javascript:alert(1)"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid nav logo = %d", status)
	}
	if status := put(`{"gravatarUrl":"data:text/html,bad"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid gravatar url = %d", status)
	}
	if status := put(`{"languageYaml":"scalar"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid language yaml = %d", status)
	}
	if status := put(`{"paginationProblem":0}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid zero pagination = %d", status)
	}
	if status := put(`{"paginationRecord":1001}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid large pagination = %d", status)
	}
	if status := put(`{"limitUserFiles":0}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid file count limit = %d", status)
	}
	if status := put(`{"limitCodeLength":0}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid code length limit = %d", status)
	}
	if status := put(`{"limitSubmission":0}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid submission rate limit = %d", status)
	}
	if status := put(`{"sessionSavedExpireSeconds":59}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid session expiry = %d", status)
	}
	updateBody := `{"bulletin":"# 欢迎来到 Test OJ","siteName":"Test OJ","navLogo":"/test-logo.png","footerExtraHtml":"<b>Test Footer</b>","enableScratchpad":false,"gravatarUrl":"https://example.com/avatar/","homepageYaml":"- width: 12\n  bulletin: true\n  contest: 3\n","aboutMarkdown":"# about 关于 {{ name }}\n\nTest About","paginationProblem":37,"paginationContest":12,"paginationDiscussion":24,"paginationRecord":48,"paginationRanking":60,"paginationSolution":18,"paginationTraining":6,"limitUserFiles":8,"limitUserFileSize":1048576,"limitContestFiles":9,"limitContestFileSize":2097152,"limitProblemFiles":10,"limitProblemFileSize":3145728,"limitCodeLength":65536,"limitSubmission":11,"limitSubmissionUser":7,"limitPretest":12,"limitByUser":true,"sessionSavedExpireSeconds":7200,"sessionUnsavedExpireSeconds":1800,"languageYaml":"cpp:\n  display: C++17\n  compile: /usr/bin/g++ -O2 -std=c++17 -o foo foo.cpp\n  code_file: foo.cpp\n  execute: /w/foo\n"}`
	if status := put(updateBody, adminCookies); status != http.StatusOK {
		t.Fatalf("admin update = %d", status)
	}
	if sessions.TTLFor("user") != 2*time.Hour || sessions.TTLFor("judge") != 30*time.Minute {
		t.Fatalf("live session TTLs = user %s judge %s", sessions.TTLFor("user"), sessions.TTLFor("judge"))
	}

	status, payload = get("/api/v1/settings", nil)
	if status != http.StatusOK || payload.Data.Bulletin != "# 欢迎来到 Test OJ" || payload.Data.SiteName != "Test OJ" ||
		payload.Data.NavLogo != "/test-logo.png" || payload.Data.FooterExtraHTML != "<b>Test Footer</b>" ||
		payload.Data.EnableScratchpad || payload.Data.GravatarURL != "https://example.com/avatar/" ||
		!strings.Contains(payload.Data.HomepageYAML, "contest: 3") || !strings.Contains(payload.Data.AboutMarkdown, "Test About") ||
		!strings.Contains(payload.Data.LanguageYAML, "C++17") ||
		payload.Data.PaginationProblem != 37 || payload.Data.PaginationContest != 12 ||
		payload.Data.PaginationDiscussion != 24 || payload.Data.PaginationRecord != 48 ||
		payload.Data.PaginationRanking != 60 || payload.Data.PaginationSolution != 18 ||
		payload.Data.PaginationTraining != 6 || payload.Data.LimitUserFiles != 8 ||
		payload.Data.LimitUserFileSize != 1048576 || payload.Data.LimitContestFiles != 9 ||
		payload.Data.LimitContestFileSize != 2097152 || payload.Data.LimitProblemFiles != 10 ||
		payload.Data.LimitProblemFileSize != 3145728 || payload.Data.LimitCodeLength != 65536 ||
		payload.Data.LimitSubmission != 11 || payload.Data.LimitSubmissionUser != 7 ||
		payload.Data.LimitPretest != 12 || !payload.Data.LimitByUser ||
		payload.Data.SessionSavedExpire != 7200 || payload.Data.SessionUnsavedExpire != 1800 {
		t.Fatalf("public settings after update = %d %+v", status, payload.Data)
	}
	status, payload = get("/api/v1/admin/settings", adminCookies)
	if status != http.StatusOK || payload.Data.SiteName != "Test OJ" || payload.Data.Bulletin != "# 欢迎来到 Test OJ" {
		t.Fatalf("admin settings = %d %+v", status, payload.Data)
	}

	if status := put(`{"bulletin":"# 部分更新"}`, adminCookies); status != http.StatusOK {
		t.Fatalf("partial update = %d", status)
	}
	status, payload = get("/api/v1/settings", nil)
	if status != http.StatusOK || payload.Data.Bulletin != "# 部分更新" || payload.Data.SiteName != "Test OJ" ||
		payload.Data.NavLogo != "/test-logo.png" || payload.Data.EnableScratchpad || payload.Data.PaginationProblem != 37 {
		t.Fatalf("partial update changed unrelated settings = %d %+v", status, payload.Data)
	}
}
