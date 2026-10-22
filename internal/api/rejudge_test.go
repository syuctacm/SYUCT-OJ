package api_test

import (
	"context"
	"fmt"
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
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestProblemRejudgeIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_rejudge_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
			"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com",
			"mailLower": "normal@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1),
		"pid": "P1001", "title": "Rejudge", "owner": int64(1), "hidden": true,
		"nSubmit": int64(2), "nAccept": int64(1),
		"config": "type: default\ntime: 1000ms\nmemory: 256m\n",
		"data":   []any{bson.M{"_id": "problem/system/1/testdata/1.in", "name": "1.in", "size": int64(3)}},
	}); err != nil {
		t.Fatal(err)
	}
	acceptedID := bson.NewObjectID()
	waID := bson.NewObjectID()
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": acceptedID, "domainId": "system", "pid": int64(1), "uid": int64(2),
			"lang": "cpp", "code": "int main(){}", "status": 1, "score": 100, "time": 10.0, "memory": 1024},
		bson.M{"_id": waID, "domainId": "system", "pid": int64(1), "uid": int64(2),
			"lang": "cpp", "code": "int main(){return 1;}", "status": 2, "score": 0, "time": 20.0, "memory": 1024},
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

	admin := loginAssetUser(t, server.URL, "admin", "secret")
	normal := loginAssetUser(t, server.URL, "normal", "secret")
	call := func(cookies []*http.Cookie) int {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/problems/P1001/rejudge", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code := call(nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous rejudge = %d", code)
	}
	if code := call(normal); code != http.StatusForbidden {
		t.Fatalf("non-admin rejudge = %d", code)
	}
	if code := call(admin); code != http.StatusOK {
		t.Fatalf("rejudge = %d", code)
	}

	var records []struct {
		ID       bson.ObjectID `bson:"_id"`
		Status   int           `bson:"status"`
		Score    int           `bson:"score"`
		Rejudged bool          `bson:"rejudged"`
	}
	cursor, err := db.Collection("record").Find(ctx, bson.M{"pid": int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := cursor.All(ctx, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d", len(records))
	}
	for _, record := range records {
		if record.Status != submission.StatusWaiting || record.Score != 0 || !record.Rejudged {
			t.Fatalf("record not reset: %#v", record)
		}
	}
	if count, _ := db.Collection("task").CountDocuments(ctx, bson.M{"pid": int64(1)}); count != 2 {
		t.Fatalf("tasks = %d", count)
	}
	var task struct {
		Source string `bson:"source"`
		Lang   string `bson:"lang"`
	}
	if err := db.Collection("task").FindOne(ctx, bson.M{"rid": acceptedID}).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if task.Source != "system/1" || task.Lang != "cpp" {
		t.Fatalf("task = %#v", task)
	}
	var pdoc struct {
		NAccept int64 `bson:"nAccept"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"docId": int64(1)}).Decode(&pdoc); err != nil {
		t.Fatal(err)
	}
	if pdoc.NAccept != 0 {
		t.Fatalf("nAccept after rejudge = %d", pdoc.NAccept)
	}

	if err := submissionRepo.ApplyJudgeResult(ctx, map[string]any{
		"rid": acceptedID.Hex(), "status": submission.StatusAccepted, "score": 100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"docId": int64(1)}).Decode(&pdoc); err != nil {
		t.Fatal(err)
	}
	if pdoc.NAccept != 1 {
		t.Fatalf("nAccept after AC = %d", pdoc.NAccept)
	}
}
