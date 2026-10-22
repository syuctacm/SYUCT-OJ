package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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

func TestSubmissionRecordEventStream(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_record_events_test_%d", time.Now().UnixNano())
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
	if _, err := db.Collection("document").InsertOne(ctx, bson.M{
		"_id": bson.NewObjectID(), "domainId": "system", "docType": 10, "docId": int64(201),
		"pid": "P201", "title": "Event problem", "owner": int64(1), "nSubmit": int64(1), "nAccept": int64(0),
		"config": "type: default\ntime: 1000ms\nmemory: 256m\n",
	}); err != nil {
		t.Fatal(err)
	}
	rid := bson.NewObjectID()
	if _, err := db.Collection("record").InsertOne(ctx, bson.M{
		"_id": rid, "uid": int64(1), "pid": int64(201), "domainId": "system",
		"lang": "cpp", "code": "int main() {}", "status": 0, "score": 0, "time": 0, "memory": 0,
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	problemRepo := problem.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, nil, userRepo)
	router := api.NewRouter(config.Config{}, db, nil, authHandler, nil, submissionHandler, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()
	cookies := loginAssetUser(t, server.URL, "admin", "secret")

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/submissions/events"
	if _, _, err := websocket.DefaultDialer.Dial(wsURL, nil); err == nil {
		t.Fatal("anonymous websocket unexpectedly connected")
	}
	header := http.Header{}
	header.Set("Cookie", cookieHeader(cookies))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial record events: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	if err := submissionRepo.ApplyJudgeResult(ctx, map[string]any{
		"rid": rid.Hex(), "status": 1, "score": 100, "time": 1.5, "memory": 1024,
	}); err != nil {
		t.Fatal(err)
	}
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read record event: %v", err)
	}
	var update submission.RecordUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		t.Fatal(err)
	}
	if update.RID != rid.Hex() || update.Status != 1 || update.Score != 100 || update.Time != 1.5 || update.Memory != 1024 {
		t.Fatalf("record event = %+v", update)
	}
}

func cookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
}
