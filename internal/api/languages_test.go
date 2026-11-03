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
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestLanguageRegistryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_languages_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	problemID := bson.NewObjectID()
	contestID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": problemID, "domainId": "system", "docType": 10, "docId": int64(2001), "pid": "P2001",
			"title": "只允许 Python", "hidden": false,
			"config": "type: default\ntime: 1000ms\nmemory: 256m\nlangs:\n  - python3\n"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2002), "pid": "P2002",
			"title": "不限语言", "hidden": false, "config": "type: default\ntime: 1000ms\nmemory: 256m\n"},
		bson.M{"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID,
			"title": "语言限制比赛", "rule": "acm", "owner": int64(1), "langs": []string{"cpp"},
			"beginAt": time.Now().Add(-time.Hour), "endAt": time.Now().Add(time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, problemRepo, nil, contestRepo, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	type languagePage struct {
		Data struct {
			Items []judge.LanguageOption `json:"items"`
		} `json:"data"`
	}
	get := func(path string) (int, languagePage) {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var page languagePage
		if resp.StatusCode == http.StatusOK {
			if err := decodeJSON(resp.Body, &page); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, page
	}

	status, page := get("/api/v1/languages")
	if status != http.StatusOK || len(page.Data.Items) == 0 {
		t.Fatalf("registry = %d %+v", status, page.Data.Items)
	}
	registry := make(map[string]string, len(page.Data.Items))
	for _, item := range page.Data.Items {
		registry[item.ID] = item.Display
	}
	if registry["cpp"] != "C++17" || registry["python3"] != "Python 3" {
		t.Fatalf("registry = %+v", registry)
	}

	status, page = get("/api/v1/problems/P2001/langs")
	if status != http.StatusOK || len(page.Data.Items) != 1 || page.Data.Items[0].ID != "python3" {
		t.Fatalf("restricted problem langs = %d %+v", status, page.Data.Items)
	}

	status, page = get("/api/v1/problems/P2002/langs")
	if status != http.StatusOK || len(page.Data.Items) != len(registry) {
		t.Fatalf("unrestricted problem langs = %d %+v", status, page.Data.Items)
	}

	status, page = get("/api/v1/problems/P2002/langs?contest=" + contestID.Hex())
	if status != http.StatusOK || len(page.Data.Items) != 1 || page.Data.Items[0].ID != "cpp" {
		t.Fatalf("contest restricted langs = %d %+v", status, page.Data.Items)
	}

	status, page = get("/api/v1/problems/P2001/langs?contest=" + contestID.Hex())
	if status != http.StatusOK || len(page.Data.Items) != 0 {
		t.Fatalf("intersection should be empty = %d %+v", status, page.Data.Items)
	}

	if status, _ := get("/api/v1/problems/P404/langs"); status != http.StatusNotFound {
		t.Fatalf("unknown problem = %d", status)
	}
}

func TestAdminLanguageConfigUpdatesLiveRegistry(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_language_setting_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(1), "uname": "admin", "unameLower": "admin",
		"mail": "admin@example.com", "mailLower": "admin@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	store := judge.NewFileStore(t.TempDir())
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	hub := judge.NewHub(judge.DefaultLanguages())
	judgeHandler := judge.NewHandler("judge", "judge", sessions, store, hub)
	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	adminHandler.SetLanguageUpdater(func(raw string) error {
		languages, err := judge.ParseLanguages(raw)
		if err != nil {
			return err
		}
		hub.SetLanguages(languages)
		return nil
	})
	router := api.NewRouter(
		config.Config{}, db, judgeHandler, authHandler, problemRepo, nil, nil, nil, nil, userRepo, adminHandler, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	languageYAML := "rust:\n  display: Rust Test\n  compile: /usr/bin/rustc -o foo foo.rs\n  code_file: foo.rs\n  execute: ./foo\n"
	body, _ := json.Marshal(map[string]string{"languageYaml": languageYAML})
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/admin/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save language config = %d", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/api/v1/languages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var page struct {
		Data struct {
			Items []judge.LanguageOption `json:"items"`
		} `json:"data"`
	}
	if err := decodeJSON(resp.Body, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data.Items) != 1 || page.Data.Items[0].ID != "rust" || page.Data.Items[0].Display != "Rust Test" {
		t.Fatalf("live language registry = %+v", page.Data.Items)
	}
	if _, ok := hub.Languages()["rust"]; !ok {
		t.Fatal("judge hub did not receive the language update")
	}
}
