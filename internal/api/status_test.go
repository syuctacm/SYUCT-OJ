package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
)

func TestStatusEndpoint(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_status_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, dbName, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()
	server := httptest.NewServer(api.NewRouter(config.Config{}, db, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	defer server.Close()
	resp, err := http.Get(server.URL + "/api/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var payload struct {
		Data struct {
			Stats []struct {
				Status   string `json:"status"`
				IsOnline bool   `json:"isOnline"`
			} `json:"stats"`
			Languages map[string]string `json:"languages"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Stats) != 1 || !payload.Data.Stats[0].IsOnline || payload.Data.Stats[0].Status != "Online" {
		t.Fatalf("stats = %+v", payload.Data.Stats)
	}
	if payload.Data.Languages["C++17 (cpp)"] == "" {
		t.Fatalf("cpp language missing: %+v", payload.Data.Languages)
	}
}
