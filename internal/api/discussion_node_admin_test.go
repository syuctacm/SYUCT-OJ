package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestDiscussionNodeAdminCRUD(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_discussion_node_admin_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(2), "uname": "alice", "unameLower": "alice", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": user.DefaultPriv},
	}); err != nil {
		t.Fatal(err)
	}

	users := user.NewRepository(db)
	repo := discussion.NewRepository(db)
	sessions, err := auth.NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	auth.NewHandler(users, sessions).RegisterRoutes(router.Group("/"))
	handler.NewDiscussionHandler(repo, sessions, users, nil).RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	userCookies := loginAssetUser(t, server.URL, "alice", "secret")
	call := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
		t.Helper()
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
		data := new(bytes.Buffer)
		_, _ = data.ReadFrom(resp.Body)
		return resp.StatusCode, data.Bytes()
	}

	if status, _ := call(http.MethodPost, "/api/v1/admin/discussion-nodes", `{"docId":"qa2","title":"问答 2","category":"探索","pic":"qa"}`, userCookies); status != http.StatusForbidden {
		t.Fatalf("non-admin create = %d", status)
	}
	status, body := call(http.MethodPost, "/api/v1/admin/discussion-nodes", `{"docId":"qa2","title":"问答 2","category":"探索","pic":"qa"}`, adminCookies)
	if status != http.StatusCreated || !bytes.Contains(body, []byte(`"docId":"qa2"`)) {
		t.Fatalf("create = %d %s", status, body)
	}
	if status, _ := call(http.MethodPost, "/api/v1/admin/discussion-nodes", `{"docId":"qa2","title":"重复","category":"探索"}`, adminCookies); status != http.StatusConflict {
		t.Fatalf("duplicate = %d", status)
	}
	if status, _ := call(http.MethodPost, "/api/v1/admin/discussion-nodes", `{"docId":"bad id","title":"坏","category":"探索"}`, adminCookies); status != http.StatusBadRequest {
		t.Fatalf("invalid id = %d", status)
	}

	status, body = call(http.MethodPatch, "/api/v1/admin/discussion-nodes/qa2", `{"title":"问答新版","category":"交流","pic":"comment"}`, adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"title":"问答新版"`)) || !bytes.Contains(body, []byte(`"category":"交流"`)) {
		t.Fatalf("update = %d %s", status, body)
	}

	status, body = call(http.MethodGet, "/api/v1/discussion-nodes", "", nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"docId":"qa2"`)) || !bytes.Contains(body, []byte(`"title":"问答新版"`)) {
		t.Fatalf("public list = %d %s", status, body)
	}

	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"domainId": "system", "docType": 21, "docId": bson.NewObjectID(),
		"parentType": 20, "parentId": "qa2", "title": "引用主题", "owner": int64(2),
	}); err != nil {
		t.Fatal(err)
	}
	if status, body := call(http.MethodDelete, "/api/v1/admin/discussion-nodes/qa2", "", adminCookies); status != http.StatusConflict || !bytes.Contains(body, []byte("still in use")) {
		t.Fatalf("delete in use = %d %s", status, body)
	}

	if status, _ := call(http.MethodPost, "/api/v1/admin/discussion-nodes", `{"docId":"unused","title":"未使用","category":"测试"}`, adminCookies); status != http.StatusCreated {
		t.Fatalf("create unused = %d", status)
	}
	if status, _ := call(http.MethodDelete, "/api/v1/admin/discussion-nodes/unused", "", adminCookies); status != http.StatusNoContent {
		t.Fatalf("delete unused = %d", status)
	}

	status, body = call(http.MethodGet, "/api/v1/admin/discussion-nodes", "", adminCookies)
	if status != http.StatusOK {
		t.Fatalf("admin list = %d %s", status, body)
	}
	var listing struct {
		Data struct {
			Items []discussion.Node `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Data.Items) != 1 || listing.Data.Items[0].DocID != "qa2" {
		t.Fatalf("admin list items = %+v", listing.Data.Items)
	}
}
