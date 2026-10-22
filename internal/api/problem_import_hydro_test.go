package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func hydroZipFixture(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	files := map[string]string{
		"pkg/problem.yaml": `pid: ABC1001
title: 导入的 Hydro 题目
tag: [导入]
difficulty: 4
limits:
  time_limit: 1
  memory: 256
`,
		"pkg/problem.md":      "# 题面\n\n求 a+b。\n",
		"pkg/testdata/1.in":   "1 2\n",
		"pkg/testdata/1.out":  "3\n",
		"pkg/solution/sol.md": "# 解法\n",
	}
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestProblemImportHydroIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_hydro_import_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com",
			"mailLower": "normal@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	store := judge.NewFileStore(t.TempDir())
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	judgeHandler := judge.NewHandler("judge", "secret", sessions, store, judge.NewHub(nil))
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		judgeHandler, authHandler, problemRepo, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	admin := loginAssetUser(t, server.URL, "admin", "secret")
	normal := loginAssetUser(t, server.URL, "normal", "secret")

	upload := func(cookies []*http.Cookie, filename string, content []byte, fields map[string]string) (int, []byte) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
			t.Fatal(err)
		}
		for key, value := range fields {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems/import/hydro", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}

	fixture := hydroZipFixture(t)
	if status, _ := upload(nil, "pkg.zip", fixture, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous import = %d", status)
	}
	if status, _ := upload(normal, "pkg.zip", fixture, nil); status != http.StatusForbidden {
		t.Fatalf("non-admin import = %d", status)
	}
	if status, _ := upload(admin, "pkg.txt", []byte("not a zip"), nil); status != http.StatusBadRequest {
		t.Fatalf("garbage import = %d", status)
	}
	if status, _ := upload(admin, "pkg.zip", fixture, map[string]string{"preferredPrefix": "123"}); status != http.StatusBadRequest {
		t.Fatalf("bad prefix import = %d", status)
	}

	status, body := upload(admin, "pkg.zip", fixture, nil)
	if status != http.StatusCreated {
		t.Fatalf("import = %d: %s", status, body)
	}
	var imported struct {
		Data struct {
			Items []struct {
				PID   string `json:"pid"`
				DocID int64  `json:"docId"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &imported); err != nil || len(imported.Data.Items) != 1 {
		t.Fatalf("import payload = %s (%v)", body, err)
	}
	pid := imported.Data.Items[0].PID
	if pid != "ABC1001" {
		t.Fatalf("pid = %q", pid)
	}

	get := func(path string) (int, []byte) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		for _, cookie := range admin {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}

	status, body = get("/api/v1/problems/" + pid)
	if status != http.StatusOK {
		t.Fatalf("detail = %d: %s", status, body)
	}
	var detail struct {
		Data struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			Tags        []string `json:"tags"`
			Difficulty  int      `json:"difficulty"`
			TimeLimit   string   `json:"timeLimit"`
			MemoryLimit string   `json:"memoryLimit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("detail payload = %s (%v)", body, err)
	}
	if detail.Data.Title != "导入的 Hydro 题目" || !strings.Contains(detail.Data.Content, "求 a+b") {
		t.Fatalf("detail = %+v", detail.Data)
	}
	if len(detail.Data.Tags) != 1 || detail.Data.Tags[0] != "导入" || detail.Data.Difficulty != 4 {
		t.Fatalf("detail tags/difficulty = %+v", detail.Data)
	}
	if detail.Data.TimeLimit != "1000ms" || detail.Data.MemoryLimit != "256m" {
		t.Fatalf("detail limits = %q %q", detail.Data.TimeLimit, detail.Data.MemoryLimit)
	}

	status, body = get("/api/v1/problems/" + pid + "/assets")
	if status != http.StatusOK {
		t.Fatalf("assets = %d: %s", status, body)
	}
	for _, want := range []string{"config.yaml", "1.in", "1.out"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Fatalf("assets miss %s: %s", want, body)
		}
	}

	status, body = get("/api/v1/problems/" + pid + "/solutions")
	if status != http.StatusOK || !bytes.Contains(body, []byte("解法")) {
		t.Fatalf("imported solution = %d: %s", status, body)
	}

	// Importing the same package again must not fail on the duplicate pid.
	status, body = upload(admin, "pkg.zip", fixture, map[string]string{"preferredPrefix": "X", "hidden": "on"})
	if status != http.StatusCreated {
		t.Fatalf("second import = %d: %s", status, body)
	}
	var second struct {
		Data struct {
			Items []struct {
				PID   string `json:"pid"`
				DocID int64  `json:"docId"`
				Title string `json:"title"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &second); err != nil || len(second.Data.Items) != 1 {
		t.Fatalf("second payload = %s (%v)", body, err)
	}
	if second.Data.Items[0].PID != "X1001" {
		t.Fatalf("prefixed pid = %q", second.Data.Items[0].PID)
	}
	status, body = get("/api/v1/problems/X1001")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"hidden":true`)) {
		t.Fatalf("hidden import = %d: %s", status, body)
	}
}
