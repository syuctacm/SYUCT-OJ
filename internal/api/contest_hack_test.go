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
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestCodeforcesContestHackAPI(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_contest_hack_api_test_%d", time.Now().UnixNano())
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
	}); err != nil {
		t.Fatal(err)
	}
	config := `type: default
time: 1000ms
memory: 256m
checker_type: testlib
checker: chk.cc
validator: val.cc
subtasks:
  - score: 100
    type: min
    cases:
      - input: 1.in
        output: 1.out
`
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
		"docId": int64(1), "pid": "PCF", "title": "Codeforces Hack", "owner": int64(1), "config": config,
		"data": []bson.M{
			{"_id": "problem/system/1/testdata/1.in", "name": "1.in", "size": int64(2)},
			{"_id": "problem/system/1/testdata/1.out", "name": "1.out", "size": int64(2)},
			{"_id": "problem/system/1/testdata/chk.cc", "name": "chk.cc", "size": int64(10)},
			{"_id": "problem/system/1/testdata/val.cc", "name": "val.cc", "size": int64(10)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	contestID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": contest.DocumentTypeContest, "docId": contestID,
		"title": "Codeforces Round", "rule": "codeforces", "owner": int64(1),
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "pids": []int64{1}, "attend": int64(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("document.status").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "docType": contest.DocumentTypeContest, "docId": contestID, "uid": int64(1), "attend": 1},
		bson.M{"domainId": "system", "docType": contest.DocumentTypeContest, "docId": contestID, "uid": int64(2), "attend": 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	submissionRepo := submission.NewRepository(db, problemRepo)
	store := judge.NewFileStore(t.TempDir())
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	submissionRepo.SetFileStore(store)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, contestRepo, userRepo)
	submissionHandler.SetFileStore(store)
	router := gin.New()
	auth.NewHandler(userRepo, sessions).RegisterRoutes(router.Group("/"))
	handler.NewContestHandler(contestRepo, contest.NewTeamRepository(db), sessions, userRepo).RegisterRoutes(router.Group("/"))
	submissionHandler.RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	alice := loginAssetUser(t, server.URL, "alice", "secret")
	aliceRecord, err := submissionRepo.Create(ctx, 1, "PCF", "cpp", "int main(){}", contestID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	bobRecord, err := submissionRepo.Create(ctx, 2, "PCF", "cpp", "int main(){}", contestID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").UpdateMany(ctx, bson.M{"_id": bson.M{"$in": []bson.ObjectID{aliceRecord.ID, bobRecord.ID}}, "contest": contestID}, bson.M{"$set": bson.M{"status": submission.StatusAccepted}}); err != nil {
		t.Fatal(err)
	}
	acmID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": acmID, "domainId": "system", "docType": contest.DocumentTypeContest, "docId": acmID,
		"title": "ACM Round", "rule": "acm", "owner": int64(1),
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "pids": []int64{1}, "attend": int64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("document.status").InsertOne(ctx, bson.M{
		"domainId": "system", "docType": contest.DocumentTypeContest, "docId": acmID, "uid": int64(1), "attend": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	acmTarget, err := submissionRepo.Create(ctx, 2, "PCF", "cpp", "int main(){}", acmID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").UpdateOne(ctx, bson.M{"_id": acmTarget.ID}, bson.M{"$set": bson.M{"status": submission.StatusAccepted}}); err != nil {
		t.Fatal(err)
	}
	do := func(path string, cookies []*http.Cookie, body string) (int, []byte) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
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
	status, body := do("/api/v1/problems/PCF/hack/"+bobRecord.ID.Hex()+"?tid="+contestID.Hex(), alice, `{"input":"1 2\n"}`)
	if status != http.StatusAccepted || !bytes.Contains(body, []byte(`"contest":"`+contestID.Hex()+`"`)) {
		t.Fatalf("codeforces contest hack = %d: %s", status, body)
	}
	status, body = do("/api/v1/problems/PCF/hack/"+aliceRecord.ID.Hex()+"?tid="+contestID.Hex(), alice, `{"input":"1 2\n"}`)
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte("your own")) {
		t.Fatalf("self contest hack = %d: %s", status, body)
	}
	status, body = do("/api/v1/problems/PCF/hack/"+acmTarget.ID.Hex()+"?tid="+acmID.Hex(), alice, `{"input":"1 2\n"}`)
	if status != http.StatusBadRequest || !bytes.Contains(body, []byte("does not allow hacks")) {
		t.Fatalf("non-codeforces contest hack = %d: %s", status, body)
	}
}
