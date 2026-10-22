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
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestHiddenProblemListAndLoginAudit(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_hidden_list_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	hash := user.HashPassword("secret", "salt")
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.Collection("user").InsertMany(ctx, []any{
		bson.M{"_id": int64(1), "uname": "admin", "unameLower": "admin", "mail": "admin@example.com",
			"mailLower": "admin@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1)},
		bson.M{"_id": int64(2), "uname": "normal", "unameLower": "normal", "mail": "normal@example.com",
			"mailLower": "normal@example.com", "salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(0),
			"loginat": old, "loginip": "1.2.3.4"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document").InsertMany(ctx, []any{
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(1),
			"pid": "P1001", "title": "Visible", "hidden": false, "tag": []string{}},
		bson.M{"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(2),
			"pid": "P2002", "title": "Hidden", "hidden": true, "tag": []string{}},
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, problemRepo, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	list := func(cookies []*http.Cookie) ([]byte, int) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/problems", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var buffer bytes.Buffer
		_, _ = buffer.ReadFrom(resp.Body)
		return buffer.Bytes(), resp.StatusCode
	}

	body, status := list(nil)
	if status != http.StatusOK {
		t.Fatalf("anonymous list = %d", status)
	}
	if !bytes.Contains(body, []byte("P1001")) || bytes.Contains(body, []byte("P2002")) {
		t.Fatalf("anonymous list = %s", body)
	}

	normal := loginAssetUser(t, server.URL, "normal", "secret")
	body, status = list(normal)
	if status != http.StatusOK {
		t.Fatalf("normal list = %d", status)
	}
	if !bytes.Contains(body, []byte("P1001")) || bytes.Contains(body, []byte("P2002")) {
		t.Fatalf("normal list = %s", body)
	}

	admin := loginAssetUser(t, server.URL, "admin", "secret")
	body, status = list(admin)
	if status != http.StatusOK {
		t.Fatalf("admin list = %d", status)
	}
	if !bytes.Contains(body, []byte("P1001")) || !bytes.Contains(body, []byte("P2002")) {
		t.Fatalf("admin list = %s", body)
	}

	var account struct {
		LoginAt time.Time `bson:"loginat"`
		LoginIP string    `bson:"loginip"`
	}
	if err := db.Collection("user").FindOne(ctx, bson.M{"_id": int64(2)}).Decode(&account); err != nil {
		t.Fatal(err)
	}
	if !account.LoginAt.After(old) {
		t.Fatalf("loginat not updated: %v", account.LoginAt)
	}
	if account.LoginIP != "127.0.0.1" {
		t.Fatalf("loginip = %q", account.LoginIP)
	}
}
