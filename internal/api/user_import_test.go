package api_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestAdminUserImportIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_user_import_test_%d", time.Now().UnixNano())
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
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com",
			"mailLower": "normal@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0)},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, nil, userRepo, adminHandler, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	admin := loginAssetUser(t, server.URL, "admin", "secret")
	normal := loginAssetUser(t, server.URL, "normal", "secret")
	call := func(cookies []*http.Cookie, body string) (int, []byte) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/admin/users/import", bytes.NewBufferString(body))
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

	payload := `{"users":"import1@example.com\timport1\tsecret123\t一号选手\t{\"school\":\"沈阳化工大学\",\"group\":\"onsite\"}\nimport2@example.com,import2,secret123","draft":true}`
	if status, _ := call(nil, payload); status != http.StatusUnauthorized {
		t.Fatalf("anonymous import = %d", status)
	}
	if status, _ := call(normal, payload); status != http.StatusForbidden {
		t.Fatalf("non-admin import = %d", status)
	}
	status, body := call(admin, payload)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"created":0`)) || !bytes.Contains(body, []byte("找到 2 个用户")) {
		t.Fatalf("draft import = %d: %s", status, body)
	}
	if count, _ := db.Collection("user").CountDocuments(ctx, bson.M{"uname": "import1"}); count != 0 {
		t.Fatal("draft created a user")
	}

	status, body = call(admin, strings.Replace(payload, `"draft":true`, `"draft":false`, 1))
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"created":2`)) {
		t.Fatalf("import = %d: %s", status, body)
	}
	var imported struct {
		ID          int64  `bson:"_id"`
		DisplayName string `bson:"displayName"`
		School      string `bson:"school"`
		Avatar      string `bson:"avatar"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"uname": "import1"}).Decode(&imported); err != nil {
		t.Fatal(err)
	}
	if imported.DisplayName != "一号选手" || imported.School != "沈阳化工大学" {
		t.Fatalf("imported profile = %#v", imported)
	}
	var group struct {
		UIDs []int64 `bson:"uids"`
	}
	if err := db.Collection("user.group").FindOne(ctx, bson.M{"domainId": "system", "name": "onsite"}).Decode(&group); err != nil {
		t.Fatal(err)
	}
	if len(group.UIDs) != 1 || group.UIDs[0] != imported.ID {
		t.Fatalf("group members = %#v", group.UIDs)
	}
	if _, _, err := loginRaw(server.URL, "import2", "secret123"); err != nil {
		t.Fatalf("imported account cannot log in: %v", err)
	}

	status, body = call(admin, payload)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"created":0`)) || !bytes.Contains(body, []byte("已注册")) {
		t.Fatalf("duplicate import = %d: %s", status, body)
	}
}
