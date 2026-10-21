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
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestTrainingCompareIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_training_compare_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com", "mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro"},
	}); err != nil {
		t.Fatal(err)
	}
	trainingID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(101),
			"pid": "P101", "title": "枚举", "difficulty": 1, "nSubmit": int64(10), "nAccept": int64(8)},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(102),
			"pid": "P102", "title": "模拟", "difficulty": 2, "nSubmit": int64(9), "nAccept": int64(5)},
		bson.M{"_id": trainingID, "domainId": "system", "docType": 40, "docId": trainingID,
			"title": "对比训练", "owner": int64(1), "attend": int64(0), "pin": false, "content": "# 基础",
			"dag": []bson.M{{"_id": int32(1), "title": "基础", "requireNids": []any{}, "pids": []any{int64(101), int64(102)}}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(7), "pid": int64(101), "status": 1, "score": 100},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(7), "pid": int64(102), "status": 2, "score": 0},
	}); err != nil {
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

	type compareResponse struct {
		Data struct {
			ViewedUID int64 `json:"viewedUid"`
			Sections  []struct {
				Problems []struct {
					DocID  int64 `json:"docId"`
					Solved bool  `json:"solved"`
				} `json:"problems"`
			} `json:"sections"`
			Compare *struct {
				Sections []struct {
					Problems []struct {
						DocID  int64 `json:"docId"`
						Solved bool  `json:"solved"`
					} `json:"problems"`
				} `json:"sections"`
			} `json:"compare"`
		} `json:"data"`
	}
	fetch := func(path string, cookies []*http.Cookie) (int, compareResponse) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body compareResponse
		if resp.StatusCode == http.StatusOK {
			if err := decodeJSON(resp.Body, &body); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, body
	}

	bobCookies := loginAssetUser(t, server.URL, "bob", "secret")
	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")

	if status, _ := fetch("/api/v1/trainings/"+trainingID.Hex()+"?uid=7", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous compare = %d", status)
	}
	if status, _ := fetch("/api/v1/trainings/"+trainingID.Hex()+"?uid=abc", bobCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid uid = %d", status)
	}

	status, body := fetch("/api/v1/trainings/"+trainingID.Hex()+"?uid=7", bobCookies)
	if status != http.StatusOK || body.Data.ViewedUID != 7 {
		t.Fatalf("compare fetch = %d viewedUid=%d", status, body.Data.ViewedUID)
	}
	if len(body.Data.Sections) != 1 || !body.Data.Sections[0].Problems[0].Solved {
		t.Fatalf("viewed sections = %+v", body.Data.Sections)
	}
	if body.Data.Compare == nil || body.Data.Compare.Sections[0].Problems[0].Solved {
		t.Fatalf("own compare sections = %+v", body.Data.Compare)
	}

	status, body = fetch("/api/v1/trainings/"+trainingID.Hex()+"?uid=7", aliceCookies)
	if status != http.StatusOK || body.Data.Compare != nil {
		t.Fatalf("self fetch should not compare = %d %+v", status, body.Data.Compare)
	}
}
