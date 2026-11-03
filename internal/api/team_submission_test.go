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
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestTeamSubmissionAndRecordAccess(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_team_submission_api_test_%d", time.Now().UnixNano())
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
	now := time.Now()
	contestID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": contest.DocumentTypeContest, "docId": contestID,
		"title": "Team Contest", "rule": "acm", "owner": int64(1), "attend": int64(1), "allowTeam": true,
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "pids": []int64{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	problemDocID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": problemDocID, "domainId": "system", "docType": 10, "docId": int64(1),
		"pid": "P1001", "title": "A+B", "owner": int64(1),
		"config": "type: default\ntime: 1000ms\nmemory: 256m\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n",
		"data": []bson.M{
			{"_id": "problem/system/1/1.in", "name": "1.in", "size": int64(2)},
			{"_id": "problem/system/1/1.out", "name": "1.out", "size": int64(2)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("document.status").InsertOne(ctx, bson.M{
		"domainId": "system", "docType": contest.DocumentTypeContest, "docId": contestID,
		"uid": int64(-1000), "attend": 1, "displayName": "Alpha", "members": []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("vuser").InsertOne(ctx, bson.M{
		"_id": int64(-1000), "uname": "team:1:alpha", "displayName": "Alpha", "members": []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	sessions := auth.NewSessionManager(time.Hour)
	router := gin.New()
	auth.NewHandler(userRepo, sessions).RegisterRoutes(router.Group("/"))
	handler.NewContestHandler(contestRepo, contest.NewTeamRepository(db), sessions, userRepo).RegisterRoutes(router.Group("/"))
	handler.NewSubmissionHandler(submissionRepo, sessions, contestRepo, userRepo).RegisterRoutes(router.Group("/"))
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

	status, body := do(http.MethodPost, "/api/v1/submissions", fmt.Sprintf(`{"pid":"P1001","lang":"cpp","code":"int main(){}","contestId":%q}`, contestID.Hex()), alice)
	if status != http.StatusAccepted || !bytes.Contains(body, []byte(`"uid":-1000`)) {
		t.Fatalf("create team submission = %d: %s", status, body)
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := decodeJSON(bytes.NewReader(body), &created); err != nil || created.Data.ID == "" {
		t.Fatalf("decode team submission: %v %s", err, body)
	}
	rid := created.Data.ID
	status, body = do(http.MethodGet, "/api/v1/submissions/"+rid, "", bob)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"code":"int main(){}"`)) || !bytes.Contains(body, []byte(`"displayName":"Alpha"`)) {
		t.Fatalf("teammate record detail = %d: %s", status, body)
	}
	status, body = do(http.MethodGet, "/api/v1/submissions/"+rid+"/code", "", bob)
	if status != http.StatusOK || string(body) != "int main(){}" {
		t.Fatalf("teammate code download = %d: %s", status, body)
	}
	status, _ = do(http.MethodGet, "/api/v1/submissions/"+rid+"/code", "", carol)
	if status != http.StatusForbidden {
		t.Fatalf("non-teammate code download = %d", status)
	}
	status, body = do(http.MethodGet, "/api/v1/submissions?tid="+contestID.Hex()+"&uidOrName=2", "", bob)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"uid":-1000`)) {
		t.Fatalf("team record list = %d: %s", status, body)
	}
}
