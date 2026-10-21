package api_test

import (
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

const importFPSFixture = `<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <item>
    <title><![CDATA[Imported A+B]]></title>
    <time_limit unit="s"><![CDATA[2]]></time_limit>
    <memory_limit unit="mb"><![CDATA[64]]></memory_limit>
    <description><![CDATA[<p>Add them.</p>]]></description>
    <input><![CDATA[Two integers.]]></input>
    <output><![CDATA[Their sum.]]></output>
    <sample_input><![CDATA[1 2]]></sample_input>
    <sample_output><![CDATA[3]]></sample_output>
    <test_input><![CDATA[1 2]]></test_input>
    <test_output><![CDATA[3]]></test_output>
    <test_input filename="second.in"><![CDATA[5 6]]></test_input>
    <test_output filename="second.out"><![CDATA[11]]></test_output>
    <source>导入 测试</source>
    <solution language="C++"><![CDATA[int main() { return 0; }]]></solution>
  </item>
</fps>`

func TestProblemImportFPSIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_problem_import_test_%d", time.Now().UnixNano())
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

	upload := func(cookies []*http.Cookie, filename, content string) (int, []byte) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems/import/fps", &body)
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

	if status, _ := upload(nil, "a.xml", importFPSFixture); status != http.StatusUnauthorized {
		t.Fatalf("anonymous import = %d", status)
	}
	if status, body := upload(normal, "a.xml", importFPSFixture); status != http.StatusForbidden {
		t.Fatalf("non-admin import = %d: %s", status, body)
	}
	if status, body := upload(admin, "a.txt", "definitely not fps"); status != http.StatusBadRequest {
		t.Fatalf("garbage import = %d: %s", status, body)
	}

	status, body := upload(admin, "a.xml", importFPSFixture)
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

	get := func(path string, cookies []*http.Cookie) (int, []byte) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
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

	status, body = get("/api/v1/problems/"+pid, admin)
	if status != http.StatusOK {
		t.Fatalf("problem detail = %d: %s", status, body)
	}
	var detail struct {
		Data struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			Html        bool     `json:"html"`
			Tags        []string `json:"tags"`
			TimeLimit   string   `json:"timeLimit"`
			MemoryLimit string   `json:"memoryLimit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("detail payload = %s (%v)", body, err)
	}
	if detail.Data.Title != "Imported A+B" || !detail.Data.Html {
		t.Fatalf("detail = %+v", detail.Data)
	}
	if !strings.Contains(detail.Data.Content, "<h2>Problem Description</h2>") ||
		!strings.Contains(detail.Data.Content, `class="language-input1"`) {
		t.Fatalf("detail content = %q", detail.Data.Content)
	}
	if len(detail.Data.Tags) != 2 || detail.Data.Tags[0] != "导入" || detail.Data.Tags[1] != "测试" {
		t.Fatalf("detail tags = %#v", detail.Data.Tags)
	}
	if detail.Data.TimeLimit != "2s" || detail.Data.MemoryLimit != "64mb" {
		t.Fatalf("detail limits = %q %q", detail.Data.TimeLimit, detail.Data.MemoryLimit)
	}

	status, body = get("/api/v1/problems/"+pid+"/assets", admin)
	if status != http.StatusOK {
		t.Fatalf("assets = %d: %s", status, body)
	}
	for _, want := range []string{"config.yaml", "1.in", "1.out", "second.in", "second.out"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Fatalf("assets miss %s: %s", want, body)
		}
	}

	status, body = get("/api/v1/problems/"+pid+"/solutions", admin)
	if status != http.StatusOK || !bytes.Contains(body, []byte("int main()")) {
		t.Fatalf("imported solution = %d: %s", status, body)
	}
}
