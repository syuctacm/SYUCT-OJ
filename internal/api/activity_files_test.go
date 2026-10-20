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
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestActivityFilesIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_activity_files_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com", "mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(7), "uname": "owner", "unameLower": "owner", "mail": "owner@example.com", "mailLower": "owner@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(8), "uname": "other", "unameLower": "other", "mail": "other@example.com", "mailLower": "other@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	contestID := bson.NewObjectID()
	trainingID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID, "title": "比赛", "rule": "acm", "owner": int64(7)},
		bson.M{"_id": trainingID, "domainId": "system", "docType": 40, "docId": trainingID, "title": "训练", "owner": int64(7), "dag": []any{}},
	}); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	store := judge.NewFileStore(root)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	userRepo := user.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	trainingRepo := training.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	handler.NewActivityFileHandler(contestRepo, store, sessions, userRepo, "contest", "contests").RegisterRoutes(router.Group("/"))
	handler.NewActivityFileHandler(trainingRepo, store, sessions, userRepo, "training", "trainings").RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	ownerCookies := loginAssetUser(t, server.URL, "owner", "secret")
	otherCookies := loginAssetUser(t, server.URL, "other", "secret")

	upload := func(path string, filename, content string, cookies []*http.Cookie) (int, []byte) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte(content))
		_ = writer.Close()
		req, _ := http.NewRequest(http.MethodPost, server.URL+path, &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode, data
	}

	status, body := upload("/api/v1/contests/"+contestID.Hex()+"/files", "note.txt", "hello", ownerCookies)
	if status != http.StatusCreated || !bytes.Contains(body, []byte(`"name":"note.txt"`)) || !bytes.Contains(body, []byte(`"size":5`)) {
		t.Fatalf("contest upload = %d: %s", status, body)
	}
	if data, err := os.ReadFile(filepath.Join(root, "contest", "system", contestID.Hex(), "public", "note.txt")); err != nil || string(data) != "hello" {
		t.Fatalf("stored contest file = %q err=%v", data, err)
	}
	status, body = upload("/api/v1/contests/"+contestID.Hex()+"/files", "bad.txt", "x", otherCookies)
	if status != http.StatusForbidden {
		t.Fatalf("non-owner upload = %d: %s", status, body)
	}
	resp, err := http.Get(server.URL + "/api/v1/contests/" + contestID.Hex() + "/files/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(downloaded) != "hello" {
		t.Fatalf("download = %d %q", resp.StatusCode, downloaded)
	}

	status, body = upload("/api/v1/trainings/"+trainingID.Hex()+"/files", "lesson.pdf", "pdf", ownerCookies)
	if status != http.StatusCreated {
		t.Fatalf("training upload = %d: %s", status, body)
	}
	if data, err := os.ReadFile(filepath.Join(root, "training", "system", trainingID.Hex(), "lesson.pdf")); err != nil || string(data) != "pdf" {
		t.Fatalf("stored training file = %q err=%v", data, err)
	}

	status, _ = upload("/api/v1/contests/"+contestID.Hex()+"/files", "../evil.txt", "bad", ownerCookies)
	if status != http.StatusCreated {
		t.Fatalf("upload filename should be sanitized to basename, got %d", status)
	}
	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/contests/"+contestID.Hex()+"/files", bytes.NewBufferString(`{"names":["note.txt"]}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range ownerCookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(root, "contest", "system", contestID.Hex(), "public", "note.txt")); !os.IsNotExist(err) {
		t.Fatalf("contest file still exists: %v", err)
	}

	var stored struct {
		Files []struct {
			Name string `bson:"name"`
		} `bson:"files"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": contestID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	for _, file := range stored.Files {
		if file.Name == "note.txt" {
			t.Fatalf("deleted file metadata remains: %+v", stored.Files)
		}
	}
	var response struct {
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	resp, err = http.Get(server.URL + "/api/v1/contests/" + contestID.Hex() + "/files")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&response)
	_ = resp.Body.Close()
	if len(response.Data.Items) != 1 || response.Data.Items[0].Name != "evil.txt" {
		t.Fatalf("files after delete = %+v", response.Data.Items)
	}
}
