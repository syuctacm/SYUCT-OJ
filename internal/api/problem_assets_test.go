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

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/problems/P5/assets/download-all", nil)
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	archiveBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("download all = %d %q", response.StatusCode, archiveBody)
	}
	archive, err := zip.NewReader(bytes.NewReader(archiveBody), int64(len(archiveBody)))
	if err != nil {
		t.Fatalf("open testdata archive: %v", err)
	}
	archiveFiles := map[string]string{}
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		archiveFiles[entry.Name] = string(content)
	}
	if len(archiveFiles) != 3 || archiveFiles["1.in"] != "7 8\n" || archiveFiles["1.out"] != "15\n" || archiveFiles["config.yaml"] != configText {
		t.Fatalf("testdata archive = %#v", archiveFiles)
	}

	renamePayload, _ := json.Marshal(map[string]any{
		"type": "testdata", "names": []string{"1.in"}, "newNames": []string{"input1.txt"},
	})
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems/P5/assets/rename", bytes.NewReader(renamePayload))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	renameBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("rename testdata = %d: %s", response.StatusCode, renameBody)
	}
	renamedContent, err := os.ReadFile(filepath.Join(storeRootPath, "problem", "system", "5", "testdata", "input1.txt"))
	if err != nil || string(renamedContent) != "7 8\n" {
		t.Fatalf("renamed content = %q err=%v", renamedContent, err)
	}
	if _, err := os.Stat(filepath.Join(storeRootPath, "problem", "system", "5", "testdata", "1.in")); !os.IsNotExist(err) {
		t.Fatalf("old testdata still exists: %v", err)
	}
	pdoc, err = problemRepo.GetForEditor(ctx, "P5")
	if err != nil {
		t.Fatal(err)
	}
	hasRenamed := false
	for _, file := range pdoc.Data {
		if file.Name == "input1.txt" {
			hasRenamed = true
		}
		if file.Name == "1.in" {
			t.Fatalf("old testdata metadata still exists: %+v", pdoc.Data)
		}
	}
	if !hasRenamed {
		t.Fatalf("renamed testdata missing from metadata: %+v", pdoc.Data)
	}

	archivePayload, _ := json.Marshal(map[string]any{
		"type": "testdata", "names": []string{"input1.txt", "1.out"},
	})
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems/P5/assets/archive", bytes.NewReader(archivePayload))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	selectedArchiveBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("selected archive = %d: %s", response.StatusCode, selectedArchiveBody)
	}
	selectedArchive, err := zip.NewReader(bytes.NewReader(selectedArchiveBody), int64(len(selectedArchiveBody)))
	if err != nil {
		t.Fatal(err)
	}
	if len(selectedArchive.File) != 2 {
		t.Fatalf("selected archive entries = %+v", selectedArchive.File)
	}

	renamePayload, _ = json.Marshal(map[string]any{
		"type": "additional", "names": []string{"notes.txt"}, "newNames": []string{"notes-renamed.txt"},
	})
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems/P5/assets/rename", bytes.NewReader(renamePayload))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range normalCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner rename = %d", response.StatusCode)
	}

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/problems/P5/assets/download-all", nil)
	for _, cookie := range normalCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner download all = %d", response.StatusCode)
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

	payload, _ := json.Marshal(map[string]any{"type": "testdata", "names": []string{"input1.txt", "1.out", "config.yaml"}})
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
	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/problems/P5/assets/download-all", nil)
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("empty testdata download = %d", response.StatusCode)
	}

	body, _ = json.Marshal(map[string]any{
		"pid": "PNEW", "title": "Created problem", "content": "# Created",
		"tags": []string{"Math", "Math", "Graph"}, "difficulty": 3, "hidden": false,
		"config": "type: default\ntime: 1000ms\nmemory: 256m\n",
	})
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	createdBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create problem = %d: %s", response.StatusCode, createdBody)
	}
	var createdEnvelope struct {
		Data struct {
			Problem struct {
				DocID int64    `json:"docId"`
				PID   string   `json:"pid"`
				Tags  []string `json:"tags"`
			} `json:"problem"`
			Config string `json:"config"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createdBody, &createdEnvelope); err != nil {
		t.Fatal(err)
	}
	if createdEnvelope.Data.Problem.DocID != 6 || createdEnvelope.Data.Problem.PID != "PNEW" || len(createdEnvelope.Data.Problem.Tags) != 2 {
		t.Fatalf("created problem: %+v", createdEnvelope.Data)
	}

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/problems/PNEW/assets/download-all", nil)
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	configArchiveBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("config-only archive = %d: %s", response.StatusCode, configArchiveBody)
	}
	configArchive, err := zip.NewReader(bytes.NewReader(configArchiveBody), int64(len(configArchiveBody)))
	if err != nil {
		t.Fatal(err)
	}
	if len(configArchive.File) != 1 || configArchive.File[0].Name != "config.yaml" {
		t.Fatalf("config-only archive entries = %+v", configArchive.File)
	}

	body, _ = json.Marshal(map[string]any{"pid": "PNEW", "title": "Duplicate"})
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate problem = %d", response.StatusCode)
	}

	body, _ = json.Marshal(map[string]any{"pid": "BAD", "title": "Bad", "config": "not: [valid"})
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid config = %d", response.StatusCode)
	}

	body, _ = json.Marshal(map[string]any{"pid": "NOPE", "title": "Nope"})
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range normalCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin create = %d", response.StatusCode)
	}

	body, _ = json.Marshal(map[string]any{
		"pid": "PNEW2", "title": "Updated problem", "content": "Updated content",
		"tags": []string{"Graph"}, "difficulty": 4, "hidden": true,
		"config": "type: default\ntime: 5000ms\nmemory: 512m\n",
	})
	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/api/v1/problems/PNEW", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	updatedBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update problem = %d: %s", response.StatusCode, updatedBody)
	}
	updatedProblem, err := problemRepo.GetForEditor(ctx, "PNEW2")
	if err != nil {
		t.Fatal(err)
	}
	if updatedProblem.Title != "Updated problem" || !updatedProblem.Hidden || updatedProblem.Difficulty != 4 || !strings.Contains(updatedProblem.Config.(string), "5000ms") {
		t.Fatalf("updated problem: %+v", updatedProblem)
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
