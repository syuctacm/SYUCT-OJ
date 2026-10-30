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

func TestContestProblemAccessAndContextIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_contest_problem_context_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(2), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1),
			"pid": "P1", "title": "Hidden Contest Problem", "hidden": true, "owner": int64(1),
		},
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2),
			"pid": "P2", "title": "Visible Contest Problem", "hidden": false, "owner": int64(1),
		},
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	liveID := bson.NewObjectID()
	upcomingID := bson.NewObjectID()
	endedID := bson.NewObjectID()
	homeworkID := bson.NewObjectID()
	unrelatedID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{
			"_id": liveID, "docId": liveID, "domainId": "system", "docType": 30,
			"title": "Live Contest", "rule": "acm", "owner": int64(1),
			"attend": int64(0), "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
			"duration": 2, "pids": []int64{1, 2},
		},
		bson.M{
			"_id": upcomingID, "docId": upcomingID, "domainId": "system", "docType": 30,
			"title": "Upcoming Contest", "rule": "acm", "owner": int64(1),
			"attend": int64(0), "beginAt": now.Add(time.Hour), "endAt": now.Add(2 * time.Hour),
			"pids": []int64{1},
		},
		bson.M{
			"_id": endedID, "docId": endedID, "domainId": "system", "docType": 30,
			"title": "Ended Contest", "rule": "acm", "owner": int64(1),
			"attend": int64(0), "beginAt": now.Add(-2 * time.Hour), "endAt": now.Add(-time.Hour),
			"pids": []int64{1},
		},
		bson.M{
			"_id": homeworkID, "docId": homeworkID, "domainId": "system", "docType": 30,
			"title": "Live Homework", "rule": "homework", "owner": int64(1),
			"attend": int64(0), "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
			"penaltySince": now.Add(time.Hour), "pids": []int64{1},
		},
		bson.M{
			"_id": unrelatedID, "docId": unrelatedID, "domainId": "system", "docType": 30,
			"title": "Unrelated Contest", "rule": "acm", "owner": int64(1),
			"attend": int64(0), "beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
			"pids": []int64{2},
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
	request := func(method, path string, body any, cookies []*http.Cookie) (int, []byte) {
		var reader io.Reader
		if body != nil {
			data, marshalErr := json.Marshal(body)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			reader = bytes.NewReader(data)
		}
		req, reqErr := http.NewRequest(method, server.URL+path, reader)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, reqErr := http.DefaultClient.Do(req)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode, data
	}
	type problemResponse struct {
		Data struct {
			PID             string                           `json:"pid"`
			Contest         *contest.Contest                 `json:"contest"`
			ContestStatus   *contest.AttendeeStatus          `json:"contestStatus"`
			ContestStatuses map[string]contest.ProblemStatus `json:"contestStatuses"`
			ContestMode     string                           `json:"contestMode"`
		} `json:"data"`
	}

	status, body := request(http.MethodGet, "/api/v1/problems/1", nil, aliceCookies)
	if status != http.StatusNotFound {
		t.Fatalf("hidden normal problem = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/problems/1?tid="+upcomingID.Hex(), nil, aliceCookies)
	if status != http.StatusForbidden || !bytes.Contains(body, []byte("not running")) {
		t.Fatalf("upcoming contest problem = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/problems/1?tid="+liveID.Hex(), nil, aliceCookies)
	if status != http.StatusForbidden || !bytes.Contains(body, []byte("attend")) {
		t.Fatalf("unattended contest problem = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/problems/1?tid="+unrelatedID.Hex(), nil, aliceCookies)
	if status != http.StatusNotFound {
		t.Fatalf("problem outside contest = %d: %s", status, body)
	}

	status, body = request(http.MethodPost, "/api/v1/contests/"+liveID.Hex()+"/attend", map[string]any{}, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("attend contest = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/problems/1?tid="+liveID.Hex(), nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("contest problem = %d: %s", status, body)
	}
	var live problemResponse
	if err := json.Unmarshal(body, &live); err != nil {
		t.Fatal(err)
	}
	if live.Data.PID != "P1" || live.Data.Contest == nil || live.Data.Contest.Rule != "acm" || live.Data.ContestMode != "contest" {
		t.Fatalf("unexpected contest problem context: %+v", live.Data)
	}
	if live.Data.ContestStatus == nil || live.Data.ContestStatus.StartAt == nil {
		t.Fatalf("contest start time was not initialized: %+v", live.Data.ContestStatus)
	}
	var storedStatus contest.AttendeeStatus
	if err := db.Collection("document.status").FindOne(ctx, bson.M{
		"domainId": "system", "docType": 30, "docId": liveID, "uid": int64(2),
	}).Decode(&storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedStatus.StartAt == nil {
		t.Fatal("stored contest status has no startAt")
	}

	status, body = request(http.MethodPost, "/api/v1/homeworks/"+homeworkID.Hex()+"/attend", nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("attend homework = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/problems/1?tid="+homeworkID.Hex(), nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("homework problem = %d: %s", status, body)
	}
	var homework problemResponse
	if err := json.Unmarshal(body, &homework); err != nil {
		t.Fatal(err)
	}
	if homework.Data.Contest == nil || homework.Data.Contest.Rule != "homework" || homework.Data.ContestMode != "contest" {
		t.Fatalf("unexpected homework problem context: %+v", homework.Data)
	}

	status, body = request(http.MethodGet, "/api/v1/problems/1?tid="+endedID.Hex(), nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("ended contest problem = %d: %s", status, body)
	}
	var ended problemResponse
	if err := json.Unmarshal(body, &ended); err != nil {
		t.Fatal(err)
	}
	if ended.Data.ContestMode != "view" || ended.Data.Contest == nil {
		t.Fatalf("unexpected ended contest context: %+v", ended.Data)
	}

	status, body = request(http.MethodPost, "/api/v1/contests/"+liveID.Hex()+"/early-end", nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("early end = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/problems/1?tid="+liveID.Hex(), nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("early-ended hidden contest problem = %d: %s", status, body)
	}
	var hiddenCorrection problemResponse
	if err := json.Unmarshal(body, &hiddenCorrection); err != nil {
		t.Fatal(err)
	}
	if hiddenCorrection.Data.ContestMode != "none" {
		t.Fatalf("early-ended hidden mode = %q, want none", hiddenCorrection.Data.ContestMode)
	}
	status, body = request(http.MethodGet, "/api/v1/problems/2?tid="+liveID.Hex(), nil, aliceCookies)
	if status != http.StatusOK {
		t.Fatalf("early-ended visible contest problem = %d: %s", status, body)
	}
	var correction problemResponse
	if err := json.Unmarshal(body, &correction); err != nil {
		t.Fatal(err)
	}
	if correction.Data.ContestMode != "correction" {
		t.Fatalf("early-ended visible mode = %q, want correction", correction.Data.ContestMode)
	}
}
