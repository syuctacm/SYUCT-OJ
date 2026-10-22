package api_test

import (
	"bytes"
	"context"
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
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestUserFilesIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_user_files_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756),
	}); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	store := judge.NewFileStore(root)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	handler.NewUserFileHandler(userRepo, store, sessions).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	upload := func(filename, content string, cookies []*http.Cookie) (int, []byte) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte(content))
		_ = writer.Close()
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/user/files", &body)
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

	if status, _ := upload("note.txt", "hello", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous upload = %d", status)
	}
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/user/files", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous list = %d", resp.StatusCode)
	}

	cookies := loginAssetUser(t, server.URL, "alice", "secret")

	status, body := upload("note.txt", "hello", cookies)
	if status != http.StatusCreated || !bytes.Contains(body, []byte(`"name":"note.txt"`)) || !bytes.Contains(body, []byte(`"size":5`)) || !bytes.Contains(body, []byte(`"downloadUrl":"/api/v1/users/7/files/note.txt"`)) {
		t.Fatalf("upload = %d: %s", status, body)
	}
	if data, err := os.ReadFile(filepath.Join(root, "user", "7", "note.txt")); err != nil || string(data) != "hello" {
		t.Fatalf("stored file = %q err=%v", data, err)
	}
	if status, body := upload("note.txt", "hello again", cookies); status != http.StatusConflict {
		t.Fatalf("duplicate upload = %d: %s", status, body)
	}

	resp, err = http.Get(server.URL + "/api/v1/user/files/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous own download = %d", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/api/v1/users/7/files/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(downloaded) != "hello" || resp.Header.Get("Cache-Control") != "public" {
		t.Fatalf("public download = %d %q cache=%q", resp.StatusCode, downloaded, resp.Header.Get("Cache-Control"))
	}

	resp, err = http.Get(server.URL + "/api/v1/users/not-a-number/files/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid public owner = %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/user/files/note.txt", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	downloaded, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(downloaded) != "hello" {
		t.Fatalf("download = %d %q", resp.StatusCode, downloaded)
	}

	if status, body := upload("../evil.txt", "bad", cookies); status != http.StatusCreated || !bytes.Contains(body, []byte(`"name":"evil.txt"`)) {
		t.Fatalf("sanitized upload = %d: %s", status, body)
	}

	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/user/files", bytes.NewBufferString(`{"names":["note.txt"]}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
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
	resp, err = http.Get(server.URL + "/api/v1/users/7/files/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted public file = %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(root, "user", "7", "note.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still exists: %v", err)
	}
	var stored struct {
		Files []struct {
			Name string `bson:"name"`
		} `bson:"_files"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Files) != 1 || stored.Files[0].Name != "evil.txt" {
		t.Fatalf("files after delete = %+v", stored.Files)
	}
}
