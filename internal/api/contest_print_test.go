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
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestPrintIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_contest_print_test_%d", time.Now().UnixNano())
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
	now := time.Now()
	contestID := bson.NewObjectID()
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": contestID, "domainId": "system", "docType": 30, "docId": contestID,
		"title": "打印比赛", "rule": "acm", "owner": int64(1), "allowPrint": true,
		"beginAt": now.Add(-time.Hour), "endAt": now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("document.status").InsertOne(ctx, bson.M{
		"domainId": "system", "docType": 30, "docId": contestID, "uid": int64(7), "attend": 1,
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	contestRepo := contest.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(
		config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, contestRepo, nil, nil, userRepo, nil, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	aliceCookies := loginAssetUser(t, server.URL, "alice", "secret")
	bobCookies := loginAssetUser(t, server.URL, "bob", "secret")

	call := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
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

	base := "/api/v1/contests/" + contestID.Hex() + "/print"
	if status, _ := call(http.MethodPost, base+"/tasks", `{"title":"main.cpp","content":"int main(){}"}`, bobCookies); status != http.StatusForbidden {
		t.Fatalf("non-attendee print = %d", status)
	}
	status, body := call(http.MethodPost, base+"/tasks", `{"title":"main.cpp","content":"int main(){}"}`, aliceCookies)
	if status != http.StatusCreated || !bytes.Contains(body, []byte(`"status":"pending"`)) {
		t.Fatalf("attendee print = %d: %s", status, body)
	}
	if status, body := call(http.MethodPost, base+"/tasks", `{"title":"x","content":""}`, aliceCookies); status != http.StatusBadRequest {
		t.Fatalf("empty content = %d: %s", status, body)
	}

	status, body = call(http.MethodGet, base+"/tasks", "", aliceCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"title":"main.cpp"`)) || bytes.Contains(body, []byte(`"isEditor":true`)) {
		t.Fatalf("attendee list = %d: %s", status, body)
	}
	if status, body := call(http.MethodGet, base+"/tasks", "", adminCookies); status != http.StatusOK || !bytes.Contains(body, []byte(`"isEditor":true`)) {
		t.Fatalf("editor list = %d: %s", status, body)
	}
	if status, _ := call(http.MethodPost, base+"/allocate", "", aliceCookies); status != http.StatusForbidden {
		t.Fatalf("attendee allocate = %d", status)
	}

	status, body = call(http.MethodPost, base+"/allocate", "", adminCookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"status":"printing"`)) {
		t.Fatalf("allocate = %d: %s", status, body)
	}
	if status, _ := call(http.MethodPost, base+"/allocate", "", adminCookies); status != http.StatusNoContent {
		t.Fatalf("second allocate = %d", status)
	}

	var task struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"docType": contest.DocumentTypeContestPrint}).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if status, body := call(http.MethodPost, base+"/tasks/"+task.ID.Hex()+"/status", `{"status":"printed"}`, adminCookies); status != http.StatusNoContent {
		t.Fatalf("update status = %d: %s", status, body)
	}
	var stored struct {
		Status string `bson:"status"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": task.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status != contest.PrintPrinted {
		t.Fatalf("stored status = %q", stored.Status)
	}
	if status, _ := call(http.MethodPost, base+"/tasks/"+bson.NewObjectID().Hex()+"/status", `{"status":"printed"}`, adminCookies); status != http.StatusNotFound {
		t.Fatalf("unknown task = %d", status)
	}
}
