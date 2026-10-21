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
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestProblemSolutionIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_problem_solution_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
			"mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(8), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com",
			"mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(9), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
			"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1001),
		"pid": "P1001", "title": "A + B", "tag": []string{"math"}, "hidden": false, "owner": int64(9),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, problemRepo, nil, nil, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	alice := loginAssetUser(t, server.URL, "alice", "secret")
	bob := loginAssetUser(t, server.URL, "bob", "secret")
	admin := loginAssetUser(t, server.URL, "admin", "secret")

	call := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
		var reader io.Reader
		if body != "" {
			reader = bytes.NewBufferString(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, reader)
		req.Header.Set("Content-Type", "application/json")
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

	// Anonymous visitors can read the list but cannot write.
	if status, _ := call(http.MethodGet, "/api/v1/problems/P1001/solutions", "", nil); status != http.StatusOK {
		t.Fatalf("anonymous list = %d", status)
	}
	if status, _ := call(http.MethodPost, "/api/v1/problems/P1001/solutions", `{"content":"hi"}`, nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous create = %d", status)
	}

	status, body := call(http.MethodPost, "/api/v1/problems/P1001/solutions", `{"content":"alice solution"}`, alice)
	if status != http.StatusCreated {
		t.Fatalf("alice create = %d: %s", status, body)
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.Data.ID == "" {
		t.Fatalf("create payload = %s (%v)", body, err)
	}
	sid := created.Data.ID

	status, body = call(http.MethodPost, "/api/v1/problems/P1001/solutions", `{"content":"bob solution"}`, bob)
	if status != http.StatusCreated {
		t.Fatalf("bob create = %d: %s", status, body)
	}
	var bobCreated struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &bobCreated)

	if status, body = call(http.MethodGet, "/api/v1/problems/P1001/solutions", "", alice); status != http.StatusOK {
		t.Fatalf("list = %d: %s", status, body)
	} else if !bytes.Contains(body, []byte(`"total":2`)) {
		t.Fatalf("list total = %s", body)
	}

	// Only the author (or an administrator) may edit or delete a solution.
	if status, _ = call(http.MethodPatch, "/api/v1/problems/P1001/solutions/"+sid, `{"content":"bob edit"}`, bob); status != http.StatusForbidden {
		t.Fatalf("bob edit alice solution = %d", status)
	}
	if status, _ = call(http.MethodPatch, "/api/v1/problems/P1001/solutions/"+sid, `{"content":"alice edit"}`, alice); status != http.StatusNoContent {
		t.Fatalf("alice edit own solution = %d", status)
	}
	if status, _ = call(http.MethodPatch, "/api/v1/problems/P1001/solutions/"+sid, `{"content":"admin edit"}`, admin); status != http.StatusNoContent {
		t.Fatalf("admin edit solution = %d", status)
	}

	// Voting keeps Hydro's delta semantics: re-voting the same value is a no-op.
	if status, body = call(http.MethodPost, "/api/v1/problems/P1001/solutions/"+sid+"/vote", `{"value":1}`, bob); status != http.StatusOK {
		t.Fatalf("bob upvote = %d: %s", status, body)
	} else if !bytes.Contains(body, []byte(`"vote":1`)) {
		t.Fatalf("upvote payload = %s", body)
	}
	if _, body = call(http.MethodPost, "/api/v1/problems/P1001/solutions/"+sid+"/vote", `{"value":1}`, bob); !bytes.Contains(body, []byte(`"vote":1`)) {
		t.Fatalf("repeat upvote payload = %s", body)
	}
	if _, body = call(http.MethodPost, "/api/v1/problems/P1001/solutions/"+sid+"/vote", `{"value":-1}`, bob); !bytes.Contains(body, []byte(`"vote":-1`)) {
		t.Fatalf("downvote payload = %s", body)
	}
	if status, _ = call(http.MethodPost, "/api/v1/problems/P1001/solutions/"+sid+"/vote", `{"value":2}`, bob); status != http.StatusBadRequest {
		t.Fatalf("invalid vote = %d", status)
	}
	if status, body = call(http.MethodGet, "/api/v1/problems/P1001/solutions", "", bob); status != http.StatusOK {
		t.Fatalf("list with votes = %d: %s", status, body)
	} else if !bytes.Contains(body, []byte(`"votes":{"`+sid+`":-1}`)) {
		t.Fatalf("votes payload = %s", body)
	}

	// Replies: authors and administrators only.
	status, body = call(http.MethodPost, "/api/v1/problems/P1001/solutions/"+sid+"/replies", `{"content":"alice reply"}`, alice)
	if status != http.StatusCreated {
		t.Fatalf("alice reply = %d: %s", status, body)
	}
	var replied struct {
		Data struct {
			Replies []struct {
				ID string `json:"id"`
			} `json:"replies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &replied); err != nil || len(replied.Data.Replies) != 1 {
		t.Fatalf("reply payload = %s (%v)", body, err)
	}
	rid := replied.Data.Replies[0].ID

	if status, _ = call(http.MethodPatch, "/api/v1/problems/P1001/solutions/"+sid+"/replies/"+rid, `{"content":"nope"}`, bob); status != http.StatusForbidden {
		t.Fatalf("bob edit alice reply = %d", status)
	}
	if status, _ = call(http.MethodPatch, "/api/v1/problems/P1001/solutions/"+sid+"/replies/"+rid, `{"content":"alice reply v2"}`, alice); status != http.StatusNoContent {
		t.Fatalf("alice edit own reply = %d", status)
	}
	if status, _ = call(http.MethodDelete, "/api/v1/problems/P1001/solutions/"+sid+"/replies/"+rid, "", bob); status != http.StatusForbidden {
		t.Fatalf("bob delete alice reply = %d", status)
	}
	if status, _ = call(http.MethodDelete, "/api/v1/problems/P1001/solutions/"+sid+"/replies/"+rid, "", admin); status != http.StatusNoContent {
		t.Fatalf("admin delete reply = %d", status)
	}
	if status, _ = call(http.MethodDelete, "/api/v1/problems/P1001/solutions/"+sid+"/replies/"+rid, "", alice); status != http.StatusNotFound {
		t.Fatalf("delete removed reply = %d", status)
	}

	// The single-solution view used by /p/:pid/solution/:sid.
	if status, body = call(http.MethodGet, "/api/v1/problems/P1001/solutions?sid="+sid, "", nil); status != http.StatusOK {
		t.Fatalf("solution detail = %d: %s", status, body)
	} else if !bytes.Contains(body, []byte(`"id":"`+sid+`"`)) {
		t.Fatalf("solution detail payload = %s", body)
	}

	// Profile statistics feed the "liked solutions" box.
	if status, body = call(http.MethodGet, "/api/v1/users/7", "", nil); status != http.StatusOK {
		t.Fatalf("profile = %d: %s", status, body)
	} else if !bytes.Contains(body, []byte(`"nSolution":1`)) {
		t.Fatalf("profile payload = %s", body)
	}

	if status, _ = call(http.MethodDelete, "/api/v1/problems/P1001/solutions/"+sid, "", bob); status != http.StatusForbidden {
		t.Fatalf("bob delete alice solution = %d", status)
	}
	if status, _ = call(http.MethodDelete, "/api/v1/problems/P1001/solutions/"+sid, "", alice); status != http.StatusNoContent {
		t.Fatalf("alice delete own solution = %d", status)
	}
	if status, body = call(http.MethodGet, "/api/v1/problems/P1001/solutions", "", nil); status != http.StatusOK {
		t.Fatalf("list after delete = %d: %s", status, body)
	} else if !bytes.Contains(body, []byte(`"total":1`)) {
		t.Fatalf("list after delete payload = %s", body)
	}
}
