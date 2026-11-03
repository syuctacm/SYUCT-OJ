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
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestPaginationSettingControlsProblemList(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_pagination_test_%d", time.Now().UnixNano())
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
		"_id": int64(1), "uname": "admin", "unameLower": "admin",
		"mail": "admin@example.com", "mailLower": "admin@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	problems := make([]any, 0, 5)
	for i := 1; i <= 5; i++ {
		problems = append(problems, bson.M{
			"_id": bson.NewObjectID(), "domainId": "system", "docType": 10,
			"docId": int64(i), "pid": fmt.Sprintf("P%d", i), "title": fmt.Sprintf("Problem %d", i),
			"hidden": false, "tag": []string{"pagination"},
		})
	}
	if _, err := db.Collection("document").InsertMany(ctx, problems); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	problemRepo := problem.NewRepository(db)
	router := api.NewRouter(
		config.Config{}, db, nil, authHandler, problemRepo, nil, nil, nil, nil, userRepo, adminHandler, sessions,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	adminCookies := loginAssetUser(t, server.URL, "admin", "secret")
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/admin/settings",
		bytes.NewBufferString(`{"paginationProblem":2}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update pagination = %d", resp.StatusCode)
	}

	type problemPage struct {
		Data struct {
			Items []any `json:"items"`
			Limit int   `json:"limit"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	get := func(path string) problemPage {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d", path, resp.StatusCode)
		}
		var page problemPage
		if err := decodeJSON(resp.Body, &page); err != nil {
			t.Fatal(err)
		}
		return page
	}

	page := get("/api/v1/problems")
	if page.Data.Limit != 2 || len(page.Data.Items) != 2 || page.Data.Total != 5 {
		t.Fatalf("default pagination = %+v", page.Data)
	}
	page = get("/api/v1/problems?limit=4")
	if page.Data.Limit != 4 || len(page.Data.Items) != 4 || page.Data.Total != 5 {
		t.Fatalf("explicit pagination = %+v", page.Data)
	}
}
