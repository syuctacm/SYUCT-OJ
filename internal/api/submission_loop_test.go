package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestSubmissionJudgeLoopIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_loop_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	_, err = db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
		"docId": int64(1), "pid": "P1001", "title": "A+B", "owner": int64(1),
		"config": "type: default\ntime: 1000ms\nmemory: 256m\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n",
		"data": []bson.M{
			{"_id": "problem/system/1/1.in", "name": "1.in", "size": int64(2)},
			{"_id": "problem/system/1/1.out", "name": "1.out", "size": int64(2)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	sessions := auth.NewSessionManager(time.Hour)
	judgeStore := judge.NewFileStore(t.TempDir())
	if err := judgeStore.Ensure(); err != nil {
		t.Fatal(err)
	}
	hub := judge.NewHub(judge.DefaultLanguages())
	judgeHandler := judge.NewHandler("judge", "judge_secret", sessions, judgeStore, hub)
	authHandler := auth.NewHandler(userRepo, sessions)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, nil)
	hub.SetResultHandler(func(payload map[string]any) {
		if err := submissionRepo.ApplyJudgeResult(context.Background(), payload); err != nil {
			t.Errorf("apply judge result: %v", err)
		}
	})
	pumpCtx, pumpCancel := context.WithCancel(context.Background())
	defer pumpCancel()
	go judge.NewTaskPump(db, hub).Run(pumpCtx)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db, judgeHandler, authHandler, problemRepo, submissionHandler, nil, nil, nil)
	server := httptest.NewServer(router)
	defer server.Close()

	userCookies := loginUser(t, server.URL)
	judgeCookie := loginJudge(t, server.URL)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/judge/conn"
	clientWS, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer " + judgeCookie.Value},
	})
	if err != nil {
		t.Fatalf("judge websocket: %v", err)
	}
	defer clientWS.Close()
	_ = clientWS.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, message, err := clientWS.ReadMessage(); err != nil || !bytes.Contains(message, []byte("language")) {
		t.Fatalf("language message: %s err=%v", message, err)
	}
	if err := clientWS.WriteJSON(map[string]string{"key": "start"}); err != nil {
		t.Fatal(err)
	}

	record := createSubmission(t, server.URL, userCookies)
	if record.ID == "" {
		t.Fatal("empty record id")
	}
	_, taskMessage, err := clientWS.ReadMessage()
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	if !bytes.Contains(taskMessage, []byte(record.ID)) || !bytes.Contains(taskMessage, []byte("P1001")) && !bytes.Contains(taskMessage, []byte("judge")) {
		t.Fatalf("task message: %s", taskMessage)
	}
	if err := clientWS.WriteJSON(map[string]any{
		"key": "end", "rid": record.ID, "status": 1, "score": 100,
		"time": 12, "memory": 1024, "message": "Accepted",
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		updated := getSubmission(t, server.URL, userCookies, record.ID)
		if updated.Status == 1 && updated.Score == 100 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("submission not updated: %+v", updated)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type recordResponse struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
	Score  int    `json:"score"`
}

func loginUser(t *testing.T, baseURL string) []*http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "secret"})
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("user login status = %d", resp.StatusCode)
	}
	return resp.Cookies()
}

func loginJudge(t *testing.T, baseURL string) *http.Cookie {
	t.Helper()
	form := url.Values{"uname": {"judge"}, "password": {"judge_secret"}}
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("judge login status = %d", resp.StatusCode)
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("judge login did not set cookie")
	}
	return cookies[0]
}

func createSubmission(t *testing.T, baseURL string, cookies []*http.Cookie) recordResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"pid": "P1001", "lang": "cpp", "code": "int main(){}"})
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/submissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create submission status = %d", resp.StatusCode)
	}
	var envelope struct {
		Data recordResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func getSubmission(t *testing.T, baseURL string, cookies []*http.Cookie, rid string) recordResponse {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/submissions/"+rid, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var envelope struct {
		Data recordResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
