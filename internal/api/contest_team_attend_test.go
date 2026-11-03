package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestTeamAttendanceAPIIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_team_attend_api_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(2), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com", "mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(3), "uname": "carol", "unameLower": "carol", "mail": "carol@example.com", "mailLower": "carol@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("user.group").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "name": "2026", "uids": []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("vuser").InsertMany(ctx, []any{
		bson.M{"_id": int64(-1000), "uname": "team:1:alpha", "displayName": "Alpha", "members": []int64{1, 2}},
		bson.M{"_id": int64(-1001), "uname": "team:3:beta", "displayName": "Beta", "members": []int64{3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	contestID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": contest.DocumentTypeContest, "docId": contestID,
		"title": "Team Contest", "rule": "acm", "owner": int64(1), "attend": int64(0), "allowTeam": true,
		"assign": []string{"2026"}, "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "pids": []int64{},
	})
	if err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	router := gin.New()
	auth.NewHandler(userRepo, sessions).RegisterRoutes(router.Group("/"))
	handler.NewContestHandler(contestRepo, contest.NewTeamRepository(db), sessions, userRepo).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	alice := loginAssetUser(t, server.URL, "alice", "secret")
	bob := loginAssetUser(t, server.URL, "bob", "secret")
	carol := loginAssetUser(t, server.URL, "carol", "secret")
	do := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result bytes.Buffer
		_, _ = result.ReadFrom(resp.Body)
		return resp.StatusCode, result.Bytes()
	}

	status, body := do(http.MethodPost, "/api/v1/contests/"+contestID.Hex()+"/attend", `{"vuid":-1000}`, alice)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"uid":-1000`)) || !bytes.Contains(body, []byte(`"members":[1,2]`)) {
		t.Fatalf("team attend = %d: %s", status, body)
	}
	status, body = do(http.MethodGet, "/api/v1/contests/"+contestID.Hex(), "", bob)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"attended":true`)) || !bytes.Contains(body, []byte(`"displayName":"Alpha"`)) {
		t.Fatalf("team attendee detail = %d: %s", status, body)
	}
	status, body = do(http.MethodGet, "/api/v1/contests/"+contestID.Hex()+"/problems", "", bob)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"attended":true`)) || !bytes.Contains(body, []byte(`"uid":-1000`)) {
		t.Fatalf("team attendee problems = %d: %s", status, body)
	}
	status, body = do(http.MethodPost, "/api/v1/contests/"+contestID.Hex()+"/attend", `{}`, bob)
	if status != http.StatusConflict {
		t.Fatalf("personal conflict = %d: %s", status, body)
	}
	status, body = do(http.MethodPost, "/api/v1/contests/"+contestID.Hex()+"/attend", `{"vuid":-1000}`, carol)
	if status != http.StatusForbidden {
		t.Fatalf("non-member team attend = %d: %s", status, body)
	}
	status, body = do(http.MethodPost, "/api/v1/contests/"+contestID.Hex()+"/attend", `{"vuid":-1001}`, carol)
	if status != http.StatusForbidden {
		t.Fatalf("unassigned team attend = %d: %s", status, body)
	}
}
