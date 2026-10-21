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

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestUserSettingsIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_user_settings_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")

	patch := func(body string, cookies []*http.Cookie) int {
		req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/user/settings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if status := patch(`{"qq":"12345"}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous settings = %d", status)
	}
	if status := patch(`{"avatar":"http://example.com/a.png"}`, aliceCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid avatar = %d", status)
	}
	if status := patch(`{"gender":5}`, aliceCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid gender = %d", status)
	}
	if status := patch(`{"backgroundImage":"url(\")"}`, aliceCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid background = %d", status)
	}

	body := `{"avatar":"gravatar:alice@example.com","qq":"123456","gender":1,"bio":"hello","school":"SYUCT","studentId":"2023001","backgroundImage":"/components/profile/backgrounds/1.jpg"}`
	if status := patch(body, aliceCookies); status != http.StatusOK {
		t.Fatalf("settings update = %d", status)
	}
	var stored user.User
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Avatar != "gravatar:alice@example.com" || stored.QQ != "123456" || stored.School != "SYUCT" ||
		stored.StudentID != "2023001" || stored.Bio != "hello" || stored.BackgroundImage != "/components/profile/backgrounds/1.jpg" {
		t.Fatalf("stored profile = %+v", stored)
	}
	if stored.Gender == nil || *stored.Gender != 1 {
		t.Fatalf("stored gender = %v", stored.Gender)
	}

	profile := func(cookies []*http.Cookie) map[string]any {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/users/7", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data
	}

	self := profile(aliceCookies)
	if self["school"] != "SYUCT" || self["studentId"] != "2023001" || self["qq"] != "123456" {
		t.Fatalf("self profile = %+v", self)
	}
	adminView := profile(adminCookies)
	if adminView["school"] != "SYUCT" || adminView["studentId"] != "2023001" {
		t.Fatalf("admin profile = %+v", adminView)
	}
	anonymous := profile(nil)
	if _, ok := anonymous["school"]; ok {
		t.Fatalf("anonymous profile leaks school: %+v", anonymous)
	}
	if _, ok := anonymous["studentId"]; ok {
		t.Fatalf("anonymous profile leaks student id: %+v", anonymous)
	}
	if anonymous["qq"] != "123456" {
		t.Fatalf("anonymous profile should expose qq: %+v", anonymous)
	}
}
