package api_test

import (
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
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestHomeListUserProgressIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_home_progress_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
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
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0),
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	contestID := bson.NewObjectID()
	homeworkID := bson.NewObjectID()
	trainingID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "One"},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2), "pid": "P2", "title": "Two"},
		bson.M{"_id": contestID, "docId": contestID, "domainId": "system", "docType": 30, "title": "Contest", "rule": "acm", "owner": int64(1), "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "pids": []int64{1}},
		bson.M{"_id": homeworkID, "docId": homeworkID, "domainId": "system", "docType": 30, "title": "Homework", "rule": "homework", "owner": int64(1), "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour), "penaltySince": now.Add(time.Hour), "pids": []int64{1}},
		bson.M{"_id": trainingID, "docId": trainingID, "domainId": "system", "docType": 40, "title": "Training", "content": "training", "owner": int64(1), "attend": int64(1), "dag": []bson.M{{"_id": int64(1), "title": "Section", "pids": []int64{1, 2}}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document.status").InsertMany(ctx, []any{
		bson.M{"domainId": "system", "docType": 30, "docId": contestID, "uid": int64(7), "attend": 1},
		bson.M{"domainId": "system", "docType": 30, "docId": homeworkID, "uid": int64(7), "attend": 1},
		bson.M{"domainId": "system", "docType": 40, "docId": trainingID, "uid": int64(7), "enroll": 1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(7), "pid": int64(1), "status": 1},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(7), "pid": int64(2), "status": 2},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	trainingRepo := training.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, contestRepo, userRepo)
	router := api.NewRouter(config.Config{}, db, nil, authHandler, problemRepo, submissionHandler, contestRepo, trainingRepo, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()
	cookies := loginAssetUser(t, server.URL, "alice", "secret")
	request := func(path string, authenticated bool) []byte {
		req, reqErr := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		if authenticated {
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
		}
		resp, reqErr := http.DefaultClient.Do(req)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s = %d: %s", path, resp.StatusCode, body)
		}
		return body
	}

	var contestPage struct {
		Data struct {
			Items []struct {
				DocID    string `json:"docId"`
				Attended bool   `json:"attended"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(request("/api/v1/contests", true), &contestPage); err != nil {
		t.Fatal(err)
	}
	if len(contestPage.Data.Items) != 1 || !contestPage.Data.Items[0].Attended {
		t.Fatalf("contest attendance missing: %+v", contestPage.Data.Items)
	}
	var anonymousContestPage struct {
		Data struct {
			Items []struct {
				Attended bool `json:"attended"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(request("/api/v1/contests", false), &anonymousContestPage); err != nil {
		t.Fatal(err)
	}
	if len(anonymousContestPage.Data.Items) != 1 || anonymousContestPage.Data.Items[0].Attended {
		t.Fatalf("anonymous contest attendance leaked: %+v", anonymousContestPage.Data.Items)
	}

	var homeworkPage struct {
		Data struct {
			Items []struct {
				Attended bool `json:"attended"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(request("/api/v1/homeworks", true), &homeworkPage); err != nil {
		t.Fatal(err)
	}
	if len(homeworkPage.Data.Items) != 1 || !homeworkPage.Data.Items[0].Attended {
		t.Fatalf("homework claim missing: %+v", homeworkPage.Data.Items)
	}

	var trainingPage struct {
		Data struct {
			Items    []training.Training              `json:"items"`
			Progress map[string]training.ListProgress `json:"progress"`
		} `json:"data"`
	}
	if err := json.Unmarshal(request("/api/v1/trainings", true), &trainingPage); err != nil {
		t.Fatal(err)
	}
	progress, ok := trainingPage.Data.Progress[trainingID.Hex()]
	if len(trainingPage.Data.Items) != 1 || !ok || !progress.Enrolled || progress.DoneProblems != 1 || progress.TotalProblems != 2 || progress.Completed {
		t.Fatalf("training progress = %+v, items=%+v", trainingPage.Data.Progress, trainingPage.Data.Items)
	}
}
