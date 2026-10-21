package api_test

import (
	"bytes"
	"context"
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
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestDiscussionHistoryIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_discussion_history_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	discussionID := bson.NewObjectID()
	replyID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": discussionID, "domainId": "system", "docType": 21, "docId": discussionID,
			"title": "历史版本", "content": "第一版", "owner": int64(7), "nReply": int64(1),
			"views": int64(1), "updateAt": time.Now().Add(-time.Hour)},
		bson.M{"_id": replyID, "domainId": "system", "docType": 22, "docId": replyID,
			"parentType": 21, "parentId": discussionID, "owner": int64(7),
			"content": "回复第一版", "reply": []any{}, "nReply": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	discussionRepo := discussion.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, discussionRepo, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	cookies := loginAssetUser(t, server.URL, "alice", "secret")

	call := func(method, path, body string) (int, []byte) {
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
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

	if status, body := call(http.MethodPatch, "/api/v1/discussions/"+discussionID.Hex(), `{"title":"历史版本","content":"第二版","pin":false,"highlight":false}`); status != http.StatusOK {
		t.Fatalf("edit topic = %d: %s", status, body)
	}
	if status, body := call(http.MethodPatch, "/api/v1/discussions/"+discussionID.Hex()+"/replies/"+replyID.Hex(), `{"content":"回复第二版"}`); status != http.StatusOK {
		t.Fatalf("edit reply = %d: %s", status, body)
	}

	type historyEnvelope struct {
		Data struct {
			History []struct {
				UID  int64     `json:"uid"`
				Time time.Time `json:"time"`
			} `json:"history"`
			Content string `json:"content"`
		} `json:"data"`
	}

	status, body := call(http.MethodGet, "/api/v1/discussions/"+discussionID.Hex()+"/raw?all=1", "")
	if status != http.StatusOK {
		t.Fatalf("topic history = %d: %s", status, body)
	}
	var history historyEnvelope
	if err := decodeJSON(bytes.NewReader(body), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Data.History) != 1 || history.Data.History[0].UID != 7 {
		t.Fatalf("topic history = %+v", history.Data.History)
	}

	ts := history.Data.History[0].Time.UnixMilli()
	status, body = call(http.MethodGet, fmt.Sprintf("/api/v1/discussions/%s/raw?time=%d", discussionID.Hex(), ts), "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"content":"第二版"`)) {
		t.Fatalf("topic revision = %d: %s", status, body)
	}
	status, body = call(http.MethodGet, "/api/v1/discussions/"+discussionID.Hex()+"/raw", "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"content":"第二版"`)) {
		t.Fatalf("topic raw = %d: %s", status, body)
	}

	status, body = call(http.MethodGet, "/api/v1/discussions/"+discussionID.Hex()+"/raw?docId="+replyID.Hex()+"&all=1", "")
	if status != http.StatusOK {
		t.Fatalf("reply history = %d: %s", status, body)
	}
	history = historyEnvelope{}
	if err := decodeJSON(bytes.NewReader(body), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Data.History) != 1 {
		t.Fatalf("reply history = %+v", history.Data.History)
	}
	replyTS := history.Data.History[0].Time.UnixMilli()
	status, body = call(http.MethodGet, fmt.Sprintf("/api/v1/discussions/%s/raw?docId=%s&time=%d", discussionID.Hex(), replyID.Hex(), replyTS), "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"content":"回复第二版"`)) {
		t.Fatalf("reply revision = %d: %s", status, body)
	}
	if status, _ := call(http.MethodGet, "/api/v1/discussions/"+discussionID.Hex()+"/raw?docId="+bson.NewObjectID().Hex(), ""); status != http.StatusNotFound {
		t.Fatalf("unknown target should 404, got %d", status)
	}
}
