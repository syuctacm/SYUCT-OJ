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
	"path/filepath"
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

func TestProblemAssetManagementIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_problem_assets_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	users := []any{
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com", "mailLower": "admin@example.com", "salt": "salt", "hash": user.HashPassword("secret", "salt"), "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com", "mailLower": "normal@example.com", "salt": "salt", "hash": user.HashPassword("secret", "salt"), "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(3), "uname": "owner", "unameLower": "owner", "mail": "owner@example.com", "mailLower": "owner@example.com", "salt": "salt", "hash": user.HashPassword("secret", "salt"), "hashType": "hydro", "priv": int64(0)},
	}
	if _, err := db.Collection("user").InsertMany(ctx, users); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
		"docId": int64(5), "pid": "P5", "title": "Asset test", "owner": int64(3),
		"config": "", "data": []any{}, "additional_file": []any{},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	storeRootPath := t.TempDir()
	store := judge.NewFileStore(storeRootPath)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	judgeHandler := judge.NewHandler("judge", "secret", sessions, store, judge.NewHub(nil))
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db, judgeHandler, authHandler, problemRepo, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	normalCookies := loginAssetUser(t, server.URL, "normal", "secret")

	status, responseBody := uploadProblemFiles(t, server.URL, "P5", "testdata", adminCookies, map[string]string{
		"1.in":  "7 8\n",
		"1.out": "15\n",
	})
	if status != http.StatusCreated {
		t.Fatalf("upload testdata = %d: %s", status, responseBody)
	}
	for filename, content := range map[string]string{"1.in": "7 8\n", "1.out": "15\n"} {
		value, err := os.ReadFile(filepath.Join(storeRootPath, "problem", "system", "5", "testdata", filename))
		if err != nil || string(value) != content {
			t.Fatalf("stored %s = %q err=%v", filename, value, err)
		}
	}

	configText := "type: default\ntime: 5000ms\nmemory: 256m\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n"
	status, responseBody = uploadProblemFiles(t, server.URL, "P5", "testdata", adminCookies, map[string]string{"config.yaml": configText})
	if status != http.StatusCreated {
		t.Fatalf("upload config = %d: %s", status, responseBody)
	}
	status, responseBody = uploadProblemFiles(t, server.URL, "P5", "additional", adminCookies, map[string]string{"notes.txt": "attachment"})
	if status != http.StatusCreated {
		t.Fatalf("upload additional = %d: %s", status, responseBody)
	}

	pdoc, err := problemRepo.GetForEditor(ctx, "P5")
	if err != nil {
		t.Fatal(err)
	}
	if len(pdoc.Data) != 3 || len(pdoc.AdditionalFiles) != 1 {
		t.Fatalf("problem files: data=%+v additional=%+v", pdoc.Data, pdoc.AdditionalFiles)
	}
	if pdoc.Config != configText {
		t.Fatalf("problem config = %q", pdoc.Config)
	}

	assets, err := getProblemAssets(server.URL, "P5", adminCookies)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets.Testdata) != 3 || len(assets.Additional) != 1 {
		t.Fatalf("asset list: %+v", assets)
	}
	var inputURL string
	for _, asset := range assets.Testdata {
		if asset.Name == "1.in" {
			inputURL = server.URL + asset.DownloadURL
		}
	}
	if inputURL == "" {
		t.Fatal("1.in download URL missing")
	}
	request, _ := http.NewRequest(http.MethodGet, inputURL, nil)
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "7 8\n" {
		t.Fatalf("download = %d %q", response.StatusCode, body)
	}

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/problems/P5/assets", nil)
	for _, cookie := range normalCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner asset access = %d", response.StatusCode)
	}

	payload, _ := json.Marshal(map[string]any{"type": "testdata", "names": []string{"1.in", "1.out", "config.yaml"}})
	request, _ = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/problems/P5/assets", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete assets = %d", response.StatusCode)
	}
	pdoc, err = problemRepo.GetForEditor(ctx, "P5")
	if err != nil {
		t.Fatal(err)
	}
	if len(pdoc.Data) != 0 || pdoc.Config != "" {
		t.Fatalf("files after delete: data=%+v config=%q", pdoc.Data, pdoc.Config)
	}
}

type problemAssetsResponse struct {
	Testdata []struct {
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		DownloadURL string `json:"downloadUrl"`
	} `json:"testdata"`
	Additional []struct {
		Name string `json:"name"`
	} `json:"additional"`
}

func loginAssetUser(t *testing.T, baseURL, username, password string) []*http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login %s = %d", username, response.StatusCode)
	}
	return response.Cookies()
}

func uploadProblemFiles(t *testing.T, baseURL, pid, kind string, cookies []*http.Cookie, files map[string]string) (int, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("type", kind); err != nil {
		t.Fatal(err)
	}
	for filename, content := range files {
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/problems/"+pid+"/assets", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(responseBody)
}

func getProblemAssets(baseURL, pid string, cookies []*http.Cookie) (*problemAssetsResponse, error) {
	request, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/problems/"+pid+"/assets", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("assets status %d: %s", response.StatusCode, body)
	}
	var envelope struct {
		Data problemAssetsResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}
