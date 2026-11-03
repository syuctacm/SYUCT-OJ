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
			PaginationProblem    int    `json:"paginationProblem"`
			PaginationContest    int    `json:"paginationContest"`
			PaginationDiscussion int    `json:"paginationDiscussion"`
			PaginationRecord     int    `json:"paginationRecord"`
			PaginationRanking    int    `json:"paginationRanking"`
			PaginationSolution   int    `json:"paginationSolution"`
			PaginationTraining   int    `json:"paginationTraining"`
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
		payload.Data.GravatarURL != handler.DefaultGravatarURL || payload.Data.HomepageYAML == "" ||
		payload.Data.PaginationProblem != 100 || payload.Data.PaginationContest != 20 ||
		payload.Data.PaginationDiscussion != 50 || payload.Data.PaginationRecord != 100 ||
		payload.Data.PaginationRanking != 100 || payload.Data.PaginationSolution != 20 ||
		payload.Data.PaginationTraining != 10 {
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
	if status := put(`{"paginationProblem":0}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid zero pagination = %d", status)
	}
	if status := put(`{"paginationRecord":1001}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid large pagination = %d", status)
	}
	updateBody := `{"bulletin":"# 欢迎来到 Test OJ","siteName":"Test OJ","navLogo":"/test-logo.png","footerExtraHtml":"<b>Test Footer</b>","enableScratchpad":false,"gravatarUrl":"https://example.com/avatar/","homepageYaml":"- width: 12\n  bulletin: true\n  contest: 3\n","aboutMarkdown":"# about 关于 {{ name }}\n\nTest About","paginationProblem":37,"paginationContest":12,"paginationDiscussion":24,"paginationRecord":48,"paginationRanking":60,"paginationSolution":18,"paginationTraining":6}`
	if status := put(updateBody, adminCookies); status != http.StatusOK {
		t.Fatalf("admin update = %d", status)
	}

	status, payload = get("/api/v1/settings", nil)
	if status != http.StatusOK || payload.Data.Bulletin != "# 欢迎来到 Test OJ" || payload.Data.SiteName != "Test OJ" ||
		payload.Data.NavLogo != "/test-logo.png" || payload.Data.FooterExtraHTML != "<b>Test Footer</b>" ||
		payload.Data.EnableScratchpad || payload.Data.GravatarURL != "https://example.com/avatar/" ||
		!strings.Contains(payload.Data.HomepageYAML, "contest: 3") || !strings.Contains(payload.Data.AboutMarkdown, "Test About") ||
		payload.Data.PaginationProblem != 37 || payload.Data.PaginationContest != 12 ||
		payload.Data.PaginationDiscussion != 24 || payload.Data.PaginationRecord != 48 ||
		payload.Data.PaginationRanking != 60 || payload.Data.PaginationSolution != 18 ||
		payload.Data.PaginationTraining != 6 {
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
