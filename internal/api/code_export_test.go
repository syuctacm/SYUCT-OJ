package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
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
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestCodeExportIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_code_export_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com", "mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	contestID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID,
		"title": "导出测试", "rule": "acm", "owner": int64(7),
		"beginAt": now.Add(-3 * time.Hour), "endAt": now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	code := func(uid int64, pid int64, lang, body string) bson.M {
		return bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "uid": uid, "pid": pid,
			"lang": lang, "status": 1, "score": 100, "code": body, "contest": contestID,
		}
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		code(7, 1001, "cpp", "int main(){}"),
		code(7, 1001, "cpp", "int main(){return 0;}"),
		code(8, 1001, "python3", "print(1)"),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, contestRepo, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")
	bobCookies := loginAssetUser(t, server.URL, "bob", "secret")

	download := func(path string, cookies []*http.Cookie) (int, http.Header, []byte) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header, body
	}

	if status, _, _ := download("/api/v1/contests/"+contestID.Hex()+"/code", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous export = %d", status)
	}
	if status, _, _ := download("/api/v1/contests/"+contestID.Hex()+"/code", bobCookies); status != http.StatusForbidden {
		t.Fatalf("non-editor export = %d", status)
	}

	status, header, body := download("/api/v1/contests/"+contestID.Hex()+"/code", aliceCookies)
	if status != http.StatusOK || header.Get("Content-Type") != "application/zip" {
		t.Fatalf("owner export = %d %s", status, header.Get("Content-Type"))
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("latest export entries = %d", len(reader.File))
	}
	names := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(handle)
		_ = handle.Close()
		names[file.Name] = string(content)
	}
	foundLatest := false
	for name, content := range names {
		if strings.HasPrefix(name, "U7_P1001_") && strings.HasSuffix(name, ".cpp") && strings.Contains(content, "return 0") {
			foundLatest = true
		}
	}
	if !foundLatest {
		t.Fatalf("latest submission missing: %+v", names)
	}

	status, _, body = download("/api/v1/contests/"+contestID.Hex()+"/code?all=1", aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("all export = %d", status)
	}
	reader, err = zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 3 {
		t.Fatalf("all export entries = %d", len(reader.File))
	}
}
