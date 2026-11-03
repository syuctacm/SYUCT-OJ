package api_test

import (
	"bytes"
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
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestTeamAPIIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_team_api_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com", "mailLower": "alice@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
		bson.M{"_id": int64(2), "uname": "bob", "unameLower": "bob", "mail": "bob@example.com", "mailLower": "bob@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}
	users := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db, nil, auth.NewHandler(users, sessions), nil, nil, nil, nil, nil, users, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	alice := loginAssetUser(t, server.URL, "alice", "secret")
	bob := loginAssetUser(t, server.URL, "bob", "secret")
	do := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data := make([]byte, 0)
		buf := bytes.NewBuffer(data)
		_, _ = buf.ReadFrom(resp.Body)
		return resp.StatusCode, buf.Bytes()
	}

	if status, _ := do(http.MethodGet, "/api/v1/contest/teams", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous team list = %d", status)
	}
	status, body := do(http.MethodPost, "/api/v1/contest/teams", `{"name":"Alpha"}`, alice)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"displayName":"Alpha"`)) {
		t.Fatalf("create team = %d %s", status, body)
	}
	status, body = do(http.MethodGet, "/api/v1/contest/teams", "", alice)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"displayName":"Alpha"`)) {
		t.Fatalf("list team = %d %s", status, body)
	}
	var snapshot struct {
		Data struct {
			Mine []struct {
				ID int64 `json:"id"`
			} `json:"mine"`
		} `json:"data"`
	}
	if err := decodeJSON(bytes.NewReader(body), &snapshot); err != nil || len(snapshot.Data.Mine) != 1 {
		t.Fatalf("decode team snapshot: %v %s", err, body)
	}
	teamID := snapshot.Data.Mine[0].ID

	status, body = do(http.MethodPost, fmt.Sprintf("/api/v1/contest/teams/%d/invite", teamID), `{"user":"bob"}`, alice)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"invite":[2]`)) {
		t.Fatalf("invite team member = %d %s", status, body)
	}
	status, body = do(http.MethodGet, "/api/v1/contest/teams", "", bob)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"displayName":"Alpha"`)) {
		t.Fatalf("invite snapshot = %d %s", status, body)
	}
	status, body = do(http.MethodPost, fmt.Sprintf("/api/v1/contest/teams/%d/accept", teamID), "", bob)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"members":[1,2]`)) {
		t.Fatalf("accept team invite = %d %s", status, body)
	}
	status, _ = do(http.MethodDelete, fmt.Sprintf("/api/v1/contest/teams/%d/members/2", teamID), "", bob)
	if status != http.StatusOK {
		t.Fatalf("leave team = %d", status)
	}
	status, body = do(http.MethodGet, "/api/v1/contest/teams", "", alice)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"members":[1]`)) {
		t.Fatalf("members after leave = %d %s", status, body)
	}
}
