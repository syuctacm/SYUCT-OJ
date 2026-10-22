package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestAttendanceAndEarlyEndIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_contest_attend_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(2), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1),
		"pid": "P1", "title": "Problem One", "hidden": false,
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	liveID := bson.NewObjectID()
	endedID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{
			"_id": liveID, "docId": liveID, "domainId": "system", "docType": 30,
			"title": "Invitation Contest", "rule": "acm", "owner": int64(1),
			"attend": int64(0), "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
			"pids": []int64{1}, "_code": "invite-me",
		},
		bson.M{
			"_id": endedID, "docId": endedID, "domainId": "system", "docType": 30,
			"title": "Ended Contest", "rule": "acm", "owner": int64(1),
			"attend": int64(0), "beginAt": now.Add(-2 * time.Hour), "endAt": now.Add(-time.Hour),
			"pids": []int64{1},
		},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, contestRepo, userRepo)
	router := api.NewRouter(config.Config{}, db, nil, authHandler, problemRepo, submissionHandler, contestRepo, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")
	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")

	request := func(method, path string, body any, cookies []*http.Cookie) (int, []byte) {
		var reader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(data)
		}
		req, err := http.NewRequest(method, server.URL+path, reader)
		if err != nil {
			t.Fatal(err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
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

	status, body := request(http.MethodGet, "/api/v1/contests/"+liveID.Hex(), nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("contest detail = %d: %s", status, body)
	}
	if bytes.Contains(body, []byte("invite-me")) || !bytes.Contains(body, []byte(`"requiresInvitationCode":true`)) {
		t.Fatalf("detail leaks or omits invitation code marker: %s", body)
	}

	status, body = request(http.MethodGet, "/api/v1/contests/"+liveID.Hex()+"/edit", nil, aliceCookies)
	if status != http.StatusForbidden {
		t.Fatalf("normal user contest edit = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/contests/"+liveID.Hex()+"/edit", nil, adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"invitationCode":"invite-me"`)) {
		t.Fatalf("owner edit invitation code = %d: %s", status, body)
	}

	status, body = request(http.MethodPost, "/api/v1/contests/"+liveID.Hex()+"/attend", nil, aliceCookies)
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte("invalid invitation code")) {
		t.Fatalf("attend without code = %d: %s", status, body)
	}
	status, body = request(http.MethodPost, "/api/v1/contests/"+liveID.Hex()+"/attend", map[string]any{"code": "wrong"}, aliceCookies)
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte("invalid invitation code")) {
		t.Fatalf("attend with wrong code = %d: %s", status, body)
	}
	status, body = request(http.MethodPost, "/api/v1/contests/"+liveID.Hex()+"/attend", map[string]any{"code": "invite-me"}, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("attend with code = %d: %s", status, body)
	}

	status, body = request(http.MethodGet, "/api/v1/contests/"+liveID.Hex(), nil, aliceCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"attend":1`)) {
		t.Fatalf("attended detail = %d: %s", status, body)
	}

	status, body = request(http.MethodPost, "/api/v1/contests/"+endedID.Hex()+"/attend", nil, aliceCookies)
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte("contest has ended")) {
		t.Fatalf("attend ended contest = %d: %s", status, body)
	}

	status, body = request(http.MethodPost, "/api/v1/contests/"+liveID.Hex()+"/early-end", nil, aliceCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"endAt"`)) {
		t.Fatalf("early end = %d: %s", status, body)
	}
	status, body = request(http.MethodPost, "/api/v1/submissions", map[string]any{
		"pid": "P1", "lang": "cpp", "code": "int main() {}", "contestId": liveID.Hex(),
	}, aliceCookies)
	if status != http.StatusForbidden || !bytes.Contains(body, []byte("contest is not running")) {
		t.Fatalf("submit after early end = %d: %s", status, body)
	}
	status, body = request(http.MethodPost, "/api/v1/contests/"+liveID.Hex()+"/early-end", nil, aliceCookies)
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte("contest has ended")) {
		t.Fatalf("second early end = %d: %s", status, body)
	}

	var stored struct {
		Attend int64 `bson:"attend"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": liveID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Attend != 1 {
		t.Fatalf("attendee counter = %d, want 1", stored.Attend)
	}
}
