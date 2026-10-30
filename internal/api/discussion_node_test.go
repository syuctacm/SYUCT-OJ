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

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/message"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestVirtualDiscussionNodes(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_discussion_nodes_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(1), "uname": "admin", "unameLower": "admin",
		"mail": "admin@example.com", "mailLower": "admin@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	contestID := bson.NewObjectID()
	trainingID := bson.NewObjectID()
	homeworkID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": "global", "docId": "global", "domainId": "system", "docType": 20, "title": "Global Node", "content": "General"},
		bson.M{"_id": contestID, "docId": contestID, "domainId": "system", "docType": 30, "title": "Contest Node", "rule": "acm", "owner": int64(1)},
		bson.M{"_id": homeworkID, "docId": homeworkID, "domainId": "system", "docType": 30, "title": "Homework Node", "rule": "homework", "owner": int64(1)},
		bson.M{"_id": trainingID, "docId": trainingID, "domainId": "system", "docType": 40, "title": "Training Node", "owner": int64(1)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	discussionRepo := discussion.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	handler.NewDiscussionHandler(discussionRepo, sessions, userRepo, message.NewRepository(db)).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()
	cookies := loginAssetUser(t, server.URL, "admin", "secret")

	request := func(method, path string, body any) (int, []byte) {
		var reader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(data)
		}
		req, err := http.NewRequest(method, server.URL+path, reader)
		if err != nil {
			t.Fatal(err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode, data
	}

	for _, test := range []struct {
		path  string
		title string
		kind  int64
	}{
		{"contest/" + contestID.Hex(), "Contest Node", 30},
		{"homework/" + homeworkID.Hex(), "Homework Node", 30},
		{"training/" + trainingID.Hex(), "Training Node", 40},
		{"node/global", "Global Node", 20},
	} {
		status, body := request(http.MethodGet, "/api/v1/discussion-nodes/"+test.path, nil)
		if status != http.StatusOK {
			t.Fatalf("node %s = %d: %s", test.path, status, body)
		}
		var envelope struct {
			Data discussion.Node `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Data.Title != test.title || envelope.Data.Type != test.kind {
			t.Fatalf("node %s = %+v", test.path, envelope.Data)
		}
	}

	status, body := request(http.MethodPost, "/api/v1/discussions", map[string]any{
		"nodeType": "contest", "nodeId": contestID.Hex(),
		"title": "Contest topic", "content": "Only in the contest node",
	})
	if status != http.StatusCreated {
		t.Fatalf("create contest discussion = %d: %s", status, body)
	}
	var created struct {
		Data discussion.Discussion `json:"data"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.ParentType != 30 || fmt.Sprint(created.Data.ParentID) != contestID.Hex() {
		t.Fatalf("created discussion parent = %d/%v", created.Data.ParentType, created.Data.ParentID)
	}
	status, body = request(http.MethodGet, "/api/v1/discussions?parentType=30&parentId="+contestID.Hex(), nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte("Contest topic")) {
		t.Fatalf("contest discussion list = %d: %s", status, body)
	}
	status, body = request(http.MethodGet, "/api/v1/discussions?parentType=40&parentId="+trainingID.Hex(), nil)
	if status != http.StatusOK || bytes.Contains(body, []byte("Contest topic")) {
		t.Fatalf("training discussion escaped its node = %d: %s", status, body)
	}
}
