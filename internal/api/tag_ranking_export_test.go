package api_test

import (
	"bytes"
	"context"
	"encoding/csv"
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
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestTagRankingExportIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_tag_ranking_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(2), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1001),
			"pid": "P1001", "title": "Math A", "tag": []string{"math"}, "hidden": false, "owner": int64(1)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1002),
			"pid": "P1002", "title": "Math B", "tag": []string{"math"}, "hidden": false, "owner": int64(1)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1003),
			"pid": "P1003", "title": "Hidden Math", "tag": []string{"math"}, "hidden": true, "owner": int64(1)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(1), "pid": int64(1001), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(1), "pid": int64(1002), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(1), "pid": int64(1001), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(1), "pid": int64(1001), "status": 2},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(2), "pid": int64(1001), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(2), "pid": int64(1001), "status": 2},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(2), "pid": int64(1002), "status": 2},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, nil, userRepo)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, problemRepo, submissionHandler, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	status, body, headers := getRaw(t, server.URL+"/api/v1/problem-tags/math/ranking/export")
	if status != http.StatusOK {
		t.Fatalf("export = %d: %s", status, body)
	}
	if !strings.Contains(headers.Get("Content-Type"), "text/csv") || !strings.Contains(headers.Get("Content-Disposition"), "attachment") {
		t.Fatalf("export headers = %v", headers)
	}
	if !bytes.HasPrefix(body, []byte("\xEF\xBB\xBF")) {
		t.Fatalf("missing UTF-8 BOM: %q", body[:min(len(body), 8)])
	}
	rows := readCSV(t, bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF")))
	if len(rows) != 3 {
		t.Fatalf("export rows = %#v", rows)
	}
	if got := strings.Join(rows[0], ","); got != "排名,UID,账号名,过题数,提交数,通过率,算法标签" {
		t.Fatalf("header = %q", got)
	}
	if got := strings.Join(rows[1], ","); got != "1,1,alice,2,4,50.0%,math" {
		t.Fatalf("alice row = %q", got)
	}
	if got := strings.Join(rows[2], ","); got != "2,2,bob,1,3,33.3%,math" {
		t.Fatalf("bob row = %q", got)
	}

	status, body, _ = getRaw(t, server.URL+"/api/v1/problem-tags/math/ranking/export?q=bob")
	if status != http.StatusOK {
		t.Fatalf("filtered export = %d: %s", status, body)
	}
	rows = readCSV(t, bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF")))
	if len(rows) != 2 || rows[1][2] != "bob" {
		t.Fatalf("filtered export rows = %#v", rows)
	}

	status, body, _ = getRaw(t, server.URL+"/api/v1/problem-tags/math/ranking?page=99&limit=1")
	if status != http.StatusOK {
		t.Fatalf("ranking page = %d: %s", status, body)
	}
	if !bytes.Contains(body, []byte(`"page":2`)) || !bytes.Contains(body, []byte(`"username":"bob"`)) {
		t.Fatalf("page clamp response = %s", body)
	}

	status, body, _ = getRaw(t, server.URL+"/api/v1/problem-tags/math/ranking?q=missing")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"matchedCount":0`)) || !bytes.Contains(body, []byte(`"items":[]`)) {
		t.Fatalf("empty search = %d: %s", status, body)
	}
}

func getRaw(t *testing.T, url string) (int, []byte, http.Header) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body, resp.Header
}

func readCSV(t *testing.T, body []byte) [][]string {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v\n%s", err, body)
	}
	return rows
}
