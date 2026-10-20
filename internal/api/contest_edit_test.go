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
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestCreateAndEditIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_contest_edit_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	users := []any{
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com", "mailLower": "admin@example.com", "salt": "salt", "hash": user.HashPassword("secret", "salt"), "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com", "mailLower": "normal@example.com", "salt": "salt", "hash": user.HashPassword("secret", "salt"), "hashType": "hydro", "priv": int64(0)},
	}
	if _, err := db.Collection("user").InsertMany(ctx, users); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1), "pid": "P1", "title": "One", "hidden": false},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2), "pid": "P2", "title": "Two", "hidden": false},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db, nil, authHandler, problemRepo, nil, contestRepo, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	normalCookies := loginAssetUser(t, server.URL, "normal", "secret")
	begin := time.Now().Add(-time.Hour).Truncate(time.Second)
	end := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	payload := map[string]any{
		"title": "Live Contest", "content": "Contest description", "rule": "acm",
		"beginAt": begin, "endAt": end, "pids": []int64{1, 2}, "rated": true, "duration": 3,
	}
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create contest = %d: %s", response.StatusCode, responseBody)
	}
	var envelope struct {
		Data struct {
			DocID string  `json:"docId"`
			Title string  `json:"title"`
			PIDs  []int64 `json:"pids"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.DocID == "" || envelope.Data.Title != "Live Contest" || len(envelope.Data.PIDs) != 2 {
		t.Fatalf("created contest: %+v", envelope.Data)
	}
	tid := envelope.Data.DocID

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/contests/"+tid+"/edit", nil)
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(responseBody, []byte("Contest description")) {
		t.Fatalf("edit contest = %d: %s", response.StatusCode, responseBody)
	}

	payload["title"] = "Updated Contest"
	payload["pids"] = []int64{2}
	payload["rule"] = "oi"
	payload["rated"] = false
	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/api/v1/contests/"+tid, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(responseBody, []byte("Updated Contest")) {
		t.Fatalf("update contest = %d: %s", response.StatusCode, responseBody)
	}
	updated, err := contestRepo.Get(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated Contest" || updated.Rule != "oi" || len(updated.PIDs) != 1 || updated.PIDs[0] != 2 {
		t.Fatalf("updated contest: %+v", updated)
	}

	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range normalCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin create contest = %d", response.StatusCode)
	}

	invalid := map[string]any{"title": "Bad", "rule": "acm", "beginAt": end, "endAt": begin, "pids": []int64{1}}
	body, _ = json.Marshal(invalid)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid contest time = %d", response.StatusCode)
	}
}
