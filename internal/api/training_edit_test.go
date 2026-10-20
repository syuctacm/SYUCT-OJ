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
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestTrainingCreateAndEditIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_training_edit_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com", "mailLower": "normal@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
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
	trainingRepo := training.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db, nil, authHandler, nil, nil, nil, trainingRepo, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	normalCookies := loginAssetUser(t, server.URL, "normal", "secret")
	payload := map[string]any{
		"title": "基础训练", "content": "从这里开始", "description": "# 训练说明", "pin": true,
		"dag": []any{
			map[string]any{"_id": 1, "title": "第一节", "requireNids": []int64{}, "pids": []any{"P1"}},
			map[string]any{"_id": 2, "title": "第二节", "requireNids": []int64{1}, "pids": []any{2}},
		},
	}
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/trainings", bytes.NewReader(body))
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
		t.Fatalf("create training = %d: %s", response.StatusCode, responseBody)
	}
	var envelope struct {
		Data struct {
			DocID string `json:"docId"`
			Title string `json:"title"`
			Pin   bool   `json:"pin"`
			DAG   []struct {
				ID   int64   `json:"id"`
				Pids []int64 `json:"pids"`
			} `json:"dag"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.DocID == "" || envelope.Data.Title != "基础训练" || !envelope.Data.Pin || len(envelope.Data.DAG) != 2 {
		t.Fatalf("created training: %+v", envelope.Data)
	}
	if len(envelope.Data.DAG[0].Pids) != 1 || envelope.Data.DAG[0].Pids[0] != 1 || envelope.Data.DAG[1].Pids[0] != 2 {
		t.Fatalf("normalized dag: %+v", envelope.Data.DAG)
	}
	tid := envelope.Data.DocID

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/trainings/"+tid+"/edit", nil)
	for _, cookie := range adminCookies {
		request.AddCookie(cookie)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(responseBody, []byte("训练说明")) {
		t.Fatalf("edit training = %d: %s", response.StatusCode, responseBody)
	}

	payload["title"] = "基础训练（更新）"
	payload["dag"] = []any{
		map[string]any{"_id": 1, "title": "第一节", "requireNids": []int64{}, "pids": []any{2}},
	}
	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/api/v1/trainings/"+tid, bytes.NewReader(body))
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
	if response.StatusCode != http.StatusOK || !bytes.Contains(responseBody, []byte("基础训练（更新）")) {
		t.Fatalf("update training = %d: %s", response.StatusCode, responseBody)
	}
	updated, err := trainingRepo.Get(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.DAG) != 1 || updated.DAG[0].Pids[0] != 2 {
		t.Fatalf("updated training: %+v", updated)
	}

	if _, err := db.Collection("document").UpdateOne(ctx, bson.M{"docId": updated.DocID}, bson.M{"$set": bson.M{"owner": int64(2)}}); err != nil {
		t.Fatal(err)
	}
	payload["title"] = "Owner Updated"
	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/api/v1/trainings/"+tid, bytes.NewReader(body))
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
		t.Fatalf("owner update training = %d", response.StatusCode)
	}

	body, _ = json.Marshal(payload)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/trainings", bytes.NewReader(body))
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
		t.Fatalf("non-admin create training = %d", response.StatusCode)
	}

	invalid := map[string]any{
		"title": "Bad", "content": "", "description": "", "pin": false,
		"dag": []any{
			map[string]any{"_id": 1, "title": "Bad", "requireNids": []int64{99}, "pids": []any{"P1"}},
		},
	}
	body, _ = json.Marshal(invalid)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/trainings", bytes.NewReader(body))
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
		t.Fatalf("invalid training dag = %d", response.StatusCode)
	}
}
