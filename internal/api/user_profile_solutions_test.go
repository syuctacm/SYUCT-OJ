package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestUserProfileRecentSolutions(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_profile_solutions_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"priv": user.DefaultPriv, "regat": time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	problems := make([]any, 0, 13)
	solutions := make([]any, 0, 13)
	addSolution := func(docID int64) {
		solutions = append(solutions, bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 11,
			"parentType": 10, "parentId": docID, "owner": int64(7), "content": "solution", "vote": int64(0),
		})
	}
	addVisibleProblems := func(start, end int64) {
		for i := start; i <= end; i++ {
			problems = append(problems, bson.M{
				"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": i,
				"pid": fmt.Sprintf("P%d", i), "title": fmt.Sprintf("Problem %d", i), "hidden": false,
			})
			addSolution(i)
		}
	}

	addVisibleProblems(101, 106)
	problems = append(problems, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(199),
		"pid": "P199", "title": "Hidden problem", "hidden": true,
	})
	addSolution(199)
	addVisibleProblems(107, 112)

	if _, err := db.Collection("document").InsertMany(ctx, problems); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertMany(ctx, solutions); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	router := api.NewRouter(
		config.Config{}, db, nil, nil, problemRepo, nil, nil, nil, nil, userRepo, nil, nil,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/users/7")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("profile status = %d", resp.StatusCode)
	}
	var envelope struct {
		Data struct {
			RecentSolutions []struct {
				ID         string `json:"id"`
				PID        int64  `json:"pid"`
				ProblemPID string `json:"problemPid"`
				Title      string `json:"title"`
			} `json:"recentSolutions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	items := envelope.Data.RecentSolutions
	if len(items) != 10 {
		t.Fatalf("recent solutions = %+v", items)
	}
	expected := []int64{112, 111, 110, 109, 108, 107, 106, 105, 104, 103}
	for i, item := range items {
		if item.ID == "" || item.PID != expected[i] || item.ProblemPID != fmt.Sprintf("P%d", expected[i]) ||
			item.Title != fmt.Sprintf("Problem %d", expected[i]) {
			t.Fatalf("recent solution %d = %+v", i, item)
		}
	}
}
