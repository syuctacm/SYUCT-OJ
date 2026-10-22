package api_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestAvatarUploadIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_avatar_test_%d", time.Now().UnixNano())
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
	handler.NewUserAvatarHandler(userRepo, store, sessions).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	if status, _ := uploadAvatar(t, server.URL, "avatar.png", pngBytes(t, 32, 32), nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous upload = %d", status)
	}

	cookies := loginAssetUser(t, server.URL, "alice", "secret")
	status, body := uploadAvatar(t, server.URL, "avatar.png", pngBytes(t, 32, 32), cookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"avatar":"/api/v1/users/7/avatar?v=`)) {
		t.Fatalf("png upload = %d: %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(root, "user", "7", ".avatar.png")); err != nil {
		t.Fatalf("stored png: %v", err)
	}
	var stored struct {
		Avatar string `bson:"avatar"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(7)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored.Avatar, "url:/api/v1/users/7/avatar?v=") {
		t.Fatalf("avatar field = %q", stored.Avatar)
	}

	resp, err := http.Get(server.URL + "/api/v1/users/7/avatar")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" || len(downloaded) == 0 {
		t.Fatalf("public avatar = %d type=%q len=%d", resp.StatusCode, resp.Header.Get("Content-Type"), len(downloaded))
	}
	if resp.Header.Get("Content-Disposition") != "inline" || !strings.Contains(resp.Header.Get("Cache-Control"), "public") {
		t.Fatalf("avatar headers = %v", resp.Header)
	}

	if status, _ := uploadAvatar(t, server.URL, "avatar.txt", []byte("not an image"), cookies); status != http.StatusBadRequest {
		t.Fatalf("invalid extension = %d", status)
	}
	if status, _ := uploadAvatar(t, server.URL, "avatar.png", []byte("not an image"), cookies); status != http.StatusBadRequest {
		t.Fatalf("invalid image = %d", status)
	}
	if status, _ := uploadAvatar(t, server.URL, "large.png", bytes.Repeat([]byte{0}, (8<<20)+1), cookies); status != http.StatusBadRequest {
		t.Fatalf("oversized image = %d", status)
	}

	status, body = uploadAvatar(t, server.URL, "portrait.jpg", jpegBytes(t, 24, 24), cookies)
	if status != http.StatusOK {
		t.Fatalf("jpeg upload = %d: %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(root, "user", "7", ".avatar.jpg")); err != nil {
		t.Fatalf("stored jpeg: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "user", "7", ".avatar.png")); !os.IsNotExist(err) {
		t.Fatalf("old png should be removed: %v", err)
	}
	resp, err = http.Get(server.URL + "/api/v1/users/7/avatar")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("jpeg public avatar = %d type=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}

func uploadAvatar(t *testing.T, baseURL, filename string, content []byte, cookies []*http.Cookie) (int, []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = writer.Close()
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/user/avatar", &body)
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

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 120, A: 255})
		}
	}
	var body bytes.Buffer
	if err := png.Encode(&body, img); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func jpegBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: uint8(x * 8), B: uint8(y * 8), A: 255})
		}
	}
	var body bytes.Buffer
	if err := jpeg.Encode(&body, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
