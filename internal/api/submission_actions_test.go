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
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestSubmissionRejudgeCancelAndCodeDownload(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_submission_actions_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(2), "uname": "owner", "unameLower": "owner", "mail": "owner@example.com", "mailLower": "owner@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(3), "uname": "other", "unameLower": "other", "mail": "other@example.com", "mailLower": "other@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	problemID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": problemID, "domainId": "system", "docType": 10, "docId": int64(201),
		"pid": "P201", "title": "Record actions", "owner": int64(2),
		"config": "type: default\ntime: 1000ms\nmemory: 256m\n", "nSubmit": int64(1), "nAccept": int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	rid := bson.NewObjectID()
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{
		"_id": rid, "uid": int64(2), "pid": int64(201), "domainId": "system",
		"lang": "cpp", "code": "int main() { return 0; }", "status": 1,
		"score": 100, "time": 10.0, "memory": 1024,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("task").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "rid": rid, "type": "judge", "pid": int64(201),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, nil, userRepo)
	router := api.NewRouter(config.Config{}, db, nil, authHandler, nil, submissionHandler, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	ownerCookies := loginAssetUser(t, server.URL, "owner", "secret")
	otherCookies := loginAssetUser(t, server.URL, "other", "secret")

	request := func(method, path string, cookies []*http.Cookie) (int, []byte) {
		req, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
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

	if status, body := request(http.MethodPost, "/api/v1/submissions/"+rid.Hex()+"/rejudge", ownerCookies); status != http.StatusForbidden {
		t.Fatalf("non-admin rejudge = %d: %s", status, body)
	}
	status, body := request(http.MethodPost, "/api/v1/submissions/"+rid.Hex()+"/rejudge", adminCookies)
	if status != http.StatusOK {
		t.Fatalf("rejudge = %d: %s", status, body)
	}
	var envelope struct {
		Data submission.Record `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Status != submission.StatusWaiting || envelope.Data.Rejudged != true {
		t.Fatalf("rejudged record = %+v", envelope.Data)
	}
	taskCount, err := db.Collection("task").CountDocuments(ctx, bson.M{"rid": rid})
	if err != nil || taskCount != 1 {
		t.Fatalf("task count after rejudge = %d err=%v", taskCount, err)
	}
	var pdoc struct {
		NAccept int64 `bson:"nAccept"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": problemID}).Decode(&pdoc); err != nil {
		t.Fatal(err)
	}
	if pdoc.NAccept != 0 {
		t.Fatalf("nAccept after rejudge = %d", pdoc.NAccept)
	}

	status, body = request(http.MethodGet, "/api/v1/submissions/"+rid.Hex()+"/code", otherCookies)
	if status != http.StatusForbidden {
		t.Fatalf("other code download = %d: %s", status, body)
	}
	for name, cookies := range map[string][]*http.Cookie{"owner": ownerCookies, "admin": adminCookies} {
		status, body = request(http.MethodGet, "/api/v1/submissions/"+rid.Hex()+"/code", cookies)
		if status != http.StatusOK || !bytes.Contains(body, []byte("int main")) {
			t.Fatalf("%s code download = %d: %s", name, status, body)
		}
	}
	status, body = request(http.MethodGet, "/api/v1/submissions/"+rid.Hex(), adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"code":"int main()`)) {
		t.Fatalf("admin record detail does not expose code = %d: %s", status, body)
	}

	status, body = request(http.MethodPost, "/api/v1/submissions/"+rid.Hex()+"/cancel", adminCookies)
	if status != http.StatusOK {
		t.Fatalf("cancel = %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Status != submission.StatusCanceled || envelope.Data.Score != 0 {
		t.Fatalf("canceled record = %+v", envelope.Data)
	}
	taskCount, err = db.Collection("task").CountDocuments(ctx, bson.M{"rid": rid})
	if err != nil || taskCount != 0 {
		t.Fatalf("task count after cancel = %d err=%v", taskCount, err)
	}
}
