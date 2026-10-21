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
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestPrivateFilesIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_contest_private_files_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(9), "uname": "attendee", "unameLower": "attendee", "mail": "attendee@example.com", "mailLower": "attendee@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	contestID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID,
		"title": "比赛", "rule": "acm", "owner": int64(7), "attend": int64(0),
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
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
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	handler.NewActivityFileHandler(contestRepo, store, sessions, userRepo, "contest", "contests").RegisterRoutes(router.Group("/"))
	handler.NewContestHandler(contestRepo, sessions, userRepo).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	ownerCookies := loginAssetUser(t, server.URL, "owner", "secret")
	otherCookies := loginAssetUser(t, server.URL, "other", "secret")
	attendeeCookies := loginAssetUser(t, server.URL, "attendee", "secret")

	upload := func(filename, content string, cookies []*http.Cookie) (int, []byte) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte(content))
		_ = writer.Close()
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests/"+contestID.Hex()+"/files?type=private", &body)
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

	status, body := upload("materials.pdf", "materials", ownerCookies)
	if status != http.StatusCreated || !bytes.Contains(body, []byte(`"name":"materials.pdf"`)) {
		t.Fatalf("private upload = %d: %s", status, body)
	}
	if !bytes.Contains(body, []byte(`"downloadUrl":"/api/v1/contests/`+contestID.Hex()+`/files/materials.pdf?type=private"`)) {
		t.Fatalf("private download url missing: %s", body)
	}
	if data, err := os.ReadFile(filepath.Join(root, "contest", "system", contestID.Hex(), "private", "materials.pdf")); err != nil || string(data) != "materials" {
		t.Fatalf("stored private file = %q err=%v", data, err)
	}
	if status, body := upload("blocked.pdf", "x", otherCookies); status != http.StatusForbidden {
		t.Fatalf("non-owner private upload = %d: %s", status, body)
	}

	privateURL := server.URL + "/api/v1/contests/" + contestID.Hex() + "/files/materials.pdf?type=private"
	download := func(cookies []*http.Cookie) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, privateURL, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode, string(data)
	}
	if status, _ := download(nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous private download = %d", status)
	}
	if status, _ := download(otherCookies); status != http.StatusForbidden {
		t.Fatalf("non-participant private download = %d", status)
	}
	if status, data := download(ownerCookies); status != http.StatusOK || data != "materials" {
		t.Fatalf("owner private download = %d %q", status, data)
	}
	if status, data := download(attendeeCookies); status != http.StatusForbidden || data == "materials" {
		t.Fatalf("non-attended participant download = %d %q", status, data)
	}

	if _, err := contestRepo.Attend(ctx, contestID.Hex(), 9); err != nil {
		t.Fatal(err)
	}
	if status, data := download(attendeeCookies); status != http.StatusOK || data != "materials" {
		t.Fatalf("attendee private download = %d %q", status, data)
	}
	if status, _ := download(otherCookies); status != http.StatusForbidden {
		t.Fatalf("unattended download after someone attended = %d", status)
	}

	listPrivate := func(cookies []*http.Cookie) int {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/contests/"+contestID.Hex()+"/files?type=private", nil)
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
	if status := listPrivate(otherCookies); status != http.StatusForbidden {
		t.Fatalf("non-owner private list = %d", status)
	}
	if status := listPrivate(ownerCookies); status != http.StatusOK {
		t.Fatalf("owner private list = %d", status)
	}

	var detail struct {
		Data struct {
			Attended     bool `json:"attended"`
			PrivateFiles []struct {
				Name string `json:"name"`
			} `json:"privateFiles"`
		} `json:"data"`
	}
	detailFor := func(cookies []*http.Cookie) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/contests/"+contestID.Hex(), nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
			t.Fatal(err)
		}
	}
	detailFor(otherCookies)
	if len(detail.Data.PrivateFiles) != 0 {
		t.Fatalf("non-participant detail leaks private files: %+v", detail.Data.PrivateFiles)
	}
	detailFor(attendeeCookies)
	if !detail.Data.Attended || len(detail.Data.PrivateFiles) != 1 || detail.Data.PrivateFiles[0].Name != "materials.pdf" {
		t.Fatalf("participant detail = %+v", detail.Data)
	}

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/contests/"+contestID.Hex()+"/files?type=private", bytes.NewBufferString(`{"names":["materials.pdf"]}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range ownerCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("private delete = %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(root, "contest", "system", contestID.Hex(), "private", "materials.pdf")); !os.IsNotExist(err) {
		t.Fatalf("private file still exists: %v", err)
	}
	if status, _ := download(ownerCookies); status != http.StatusNotFound {
		t.Fatalf("deleted private download = %d", status)
	}
}
