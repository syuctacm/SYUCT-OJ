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
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestProblemStatisticsIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_problem_statistics_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(2), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1001),
		"pid": "P1001", "title": "Stats", "tag": []string{}, "hidden": false, "owner": int64(1),
		"config": "time: 1s\nmemory: 256mb\n", "data": []any{}, "additional_file": []any{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("record").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(2), "pid": int64(1001),
			"lang": "cpp", "code": "int main(){}", "status": 1, "score": 100, "time": 10.0, "memory": 1000},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(2), "pid": int64(1001),
			"lang": "cpp", "code": "print(1)", "status": 2, "score": 40, "time": 30.0, "memory": 2000},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "uid": int64(2), "pid": int64(1001),
			"lang": "python", "code": "print(1)\n", "status": 1, "score": 100, "time": 20.0, "memory": 1500},
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

	cookies := loginAssetUser(t, server.URL, "admin", "secret")
	get := func(path string) (int, []byte) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}

	var payload struct {
		Data struct {
			Total     int64            `json:"total"`
			Sort      string           `json:"sort"`
			Direction int              `json:"direction"`
			Stats     map[string]int64 `json:"stats"`
			Items     []struct {
				ID     string  `json:"id"`
				Lang   string  `json:"lang"`
				Status int     `json:"status"`
				Score  int     `json:"score"`
				Time   float64 `json:"time"`
				Length int     `json:"length"`
			} `json:"items"`
		} `json:"data"`
	}

	status, body := get("/api/v1/problems/P1001/statistics")
	if status != http.StatusOK {
		t.Fatalf("statistics = %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload = %s (%v)", body, err)
	}
	if payload.Data.Total != 3 || len(payload.Data.Items) != 3 {
		t.Fatalf("total = %d items = %d", payload.Data.Total, len(payload.Data.Items))
	}
	if payload.Data.Sort != "time" || payload.Data.Direction != 1 {
		t.Fatalf("sort = %q direction = %d", payload.Data.Sort, payload.Data.Direction)
	}
	if payload.Data.Items[0].Time != 10 || payload.Data.Items[2].Time != 30 {
		t.Fatalf("ascending time order = %+v", payload.Data.Items)
	}
	if payload.Data.Items[0].Length != len("int main(){}") {
		t.Fatalf("length = %d", payload.Data.Items[0].Length)
	}
	for key, want := range map[string]int64{"total": 3, "AC": 2, "WA": 1, "s100": 2, "s40": 1, "s0": 0} {
		if payload.Data.Stats[key] != want {
			t.Fatalf("stats[%s] = %d, want %d (%v)", key, payload.Data.Stats[key], want, payload.Data.Stats)
		}
	}

	status, body = get("/api/v1/problems/P1001/statistics?direction=-1")
	if status != http.StatusOK {
		t.Fatalf("descending = %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Items[0].Time != 30 || payload.Data.Items[2].Time != 10 {
		t.Fatalf("descending time order = %+v", payload.Data.Items)
	}

	status, body = get("/api/v1/problems/P1001/statistics?lang=python")
	if status != http.StatusOK {
		t.Fatalf("lang filter = %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Total != 1 || len(payload.Data.Items) != 1 || payload.Data.Items[0].Lang != "python" {
		t.Fatalf("lang filter payload = %s", body)
	}

	status, body = get("/api/v1/problems/P1001/statistics?sort=memory&direction=-1")
	if status != http.StatusOK {
		t.Fatalf("memory sort = %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Items[0].Score != 40 {
		t.Fatalf("memory descending order = %+v", payload.Data.Items)
	}

	if status, _ := get("/api/v1/problems/P9999/statistics"); status != http.StatusNotFound {
		t.Fatalf("missing problem = %d", status)
	}
}
