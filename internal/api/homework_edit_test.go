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

func TestHomeworkCreateAndEditIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_homework_edit_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com", "mailLower": "admin@example.com", "salt": "salt", "hash": user.HashPassword("secret", "salt"), "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com", "mailLower": "normal@example.com", "salt": "salt", "hash": user.HashPassword("secret", "salt"), "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
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
	penalty := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	end := penalty.Add(24 * time.Hour)
	payload := map[string]any{
		"title": "Week 1", "content": "Read the statement", "beginAt": begin, "endAt": end,
		"penaltySince": penalty, "pids": []int64{1, 2}, "rated": false, "duration": 49,
		"penaltyRules": map[string]float64{"1": 0.9, "3": 0.8},
		"maintainer":   []int64{2}, "assign": []string{"2026"}, "langs": []string{"cpp", "python"},
	}
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/homeworks", bytes.NewReader(body))
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
		t.Fatalf("create homework = %d: %s", response.StatusCode, responseBody)
	}
	var envelope struct {
		Data struct {
			DocID  string   `json:"docId"`
			Rule   string   `json:"rule"`
			Title  string   `json:"title"`
			Assign []string `json:"assign"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.DocID == "" || envelope.Data.Rule != "homework" || envelope.Data.Title != "Week 1" || len(envelope.Data.Assign) != 1 {
		t.Fatalf("created homework: %+v", envelope.Data)
	}
	tid := envelope.Data.DocID

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/homeworks/"+tid+"/edit", nil)
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(responseBody, []byte("Read the statement")) {
		t.Fatalf("edit homework = %d: %s", response.StatusCode, responseBody)
	}

	payload["title"] = "Week 1 Updated"
	payload["pids"] = []int64{2}
	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/api/v1/homeworks/"+tid, bytes.NewReader(body))
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
	if response.StatusCode != http.StatusOK || !bytes.Contains(responseBody, []byte("Week 1 Updated")) {
		t.Fatalf("update homework = %d: %s", response.StatusCode, responseBody)
	}
	updated, err := contestRepo.Get(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Week 1 Updated" || len(updated.PIDs) != 1 || updated.PIDs[0] != 2 || updated.PenaltySince == nil || len(updated.PenaltyRules) != 2 {
		t.Fatalf("updated homework: %+v", updated)
	}

	if _, err := db.Collection("document").UpdateOne(ctx, bson.M{"docId": updated.DocID}, bson.M{"$set": bson.M{"owner": int64(2)}}); err != nil {
		t.Fatal(err)
	}
	payload["title"] = "Owner Updated"
	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/api/v1/homeworks/"+tid, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range normalCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("owner update homework = %d", response.StatusCode)
	}

	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/homeworks", bytes.NewReader(body))
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
		t.Fatalf("non-admin create homework = %d", response.StatusCode)
	}

	invalid := map[string]any{"title": "Bad", "beginAt": penalty, "endAt": begin, "pids": []int64{1}}
	body, _ = json.Marshal(invalid)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/homeworks", bytes.NewReader(body))
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
		t.Fatalf("invalid homework time = %d", response.StatusCode)
	}
}
