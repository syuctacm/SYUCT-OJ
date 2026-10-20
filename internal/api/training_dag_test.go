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
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestTrainingDAGIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_training_api_test_%d", time.Now().UnixNano())
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
	_, err = db.Collection("document").InsertMany(ctx, []any{
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(101),
			"pid": "P101", "title": "枚举", "difficulty": 1, "nSubmit": int64(10), "nAccept": int64(8),
		},
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(102),
			"pid": "P102", "title": "模拟", "difficulty": 2, "nSubmit": int64(12), "nAccept": int64(7),
		},
		bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(103),
			"pid": "P103", "title": "二分", "difficulty": 3, "nSubmit": int64(8), "nAccept": int64(4),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	trainingID := bson.NewObjectID()
	_, err = db.Collection("document").InsertOne(ctx, bson.M{
		"_id": trainingID, "domainId": "system", "docType": 40, "docId": trainingID,
		"title": "基础训练", "owner": int64(1), "attend": int64(0), "pin": false,
		"content": "# 第一阶段", "dag": []bson.M{
			{"_id": int32(1), "title": "基础", "requireNids": []any{}, "pids": []any{int64(101), "102"}},
			{"_id": int32(2), "title": "进阶", "requireNids": []any{int32(1)}, "pids": []any{int64(103)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(101), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(102), "domainId": "system", "status": 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	trainingRepo := training.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, trainingRepo, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/trainings/"+trainingID.Hex()+"/enroll", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated enroll status = %d", resp.StatusCode)
	}

	cookies := loginDiscussionUser(t, server.URL)
	before := getTrainingDetail(t, server.URL, trainingID.Hex(), cookies)
	if before.Enrolled || len(before.Sections) != 2 {
		t.Fatalf("unexpected unenrolled detail: %+v", before)
	}
	if before.Sections[0].Status != "progress" || before.Sections[0].Problems[0].PID != "P101" || !before.Sections[0].Problems[0].Solved {
		t.Fatalf("unexpected first section: %+v", before.Sections[0])
	}
	if before.Sections[1].Status != "invalid" {
		t.Fatalf("unexpected second section: %+v", before.Sections[1])
	}

	enrollTraining(t, server.URL, cookies, trainingID.Hex(), true)
	enrollTraining(t, server.URL, cookies, trainingID.Hex(), false)
	afterEnroll := getTrainingDetail(t, server.URL, trainingID.Hex(), cookies)
	if !afterEnroll.Enrolled || afterEnroll.Attend != 1 {
		t.Fatalf("enrollment not persisted: %+v", afterEnroll)
	}
	var trainingDoc struct {
		Attend int64 `bson:"attend"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": trainingID}).Decode(&trainingDoc); err != nil {
		t.Fatal(err)
	}
	if trainingDoc.Attend != 1 {
		t.Fatalf("attend = %d, want 1", trainingDoc.Attend)
	}

	_, err = db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(102), "domainId": "system", "status": 1},
		bson.M{"_id": bson.NewObjectID(), "uid": int64(7), "pid": int64(103), "domainId": "system", "status": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	completed := getTrainingDetail(t, server.URL, trainingID.Hex(), cookies)
	if !completed.Completed || completed.DoneProblems != 3 || completed.TotalProblems != 3 {
		t.Fatalf("unexpected completion: %+v", completed)
	}
	for _, section := range completed.Sections {
		if section.Status != "done" || section.Progress != 100 {
			t.Fatalf("section not done: %+v", section)
		}
	}
}

type trainingDetailResponse struct {
	Attend        int64 `json:"attend"`
	Enrolled      bool  `json:"enrolled"`
	Completed     bool  `json:"completed"`
	DoneProblems  int   `json:"doneProblems"`
	TotalProblems int   `json:"totalProblems"`
	Sections      []struct {
		ID       int64  `json:"id"`
		Status   string `json:"status"`
		Progress int    `json:"progress"`
		Problems []struct {
			PID    string `json:"pid"`
			Solved bool   `json:"solved"`
		} `json:"problems"`
	} `json:"sections"`
}

func getTrainingDetail(t *testing.T, baseURL, tid string, cookies []*http.Cookie) trainingDetailResponse {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/trainings/"+tid, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("training detail status = %d", resp.StatusCode)
	}
	var envelope struct {
		Data trainingDetailResponse `json:"data"`
	}
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func enrollTraining(t *testing.T, baseURL string, cookies []*http.Cookie, tid string, wantChanged bool) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/trainings/"+tid+"/enroll", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enroll status = %d", resp.StatusCode)
	}
	var envelope struct {
		Data struct {
			Changed bool `json:"changed"`
		} `json:"data"`
	}
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Changed != wantChanged {
		t.Fatalf("enroll changed = %v, want %v", envelope.Data.Changed, wantChanged)
	}
}

func decodeJSON(reader io.Reader, target any) error {
	return json.NewDecoder(reader).Decode(target)
}
