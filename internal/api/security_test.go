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

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestHomeSecurityIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_security_test_%d", time.Now().UnixNano())
	client, db, err := database.Connect(ctx, uri, name, 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	}()

	salt := "salt"
	if _, err := db.Collection("user").InsertOne(ctx, bson.M{
		"_id": int64(7), "uname": "alice", "unameLower": "alice", "mail": "alice@example.com",
		"mailLower": "alice@example.com", "salt": salt, "hash": user.HashPassword("secret", salt),
		"hashType": "hydro", "priv": int64(0),
	}); err != nil {
		t.Fatal(err)
	}

	userRepo := user.NewRepository(db)
	sessions, err := auth.NewMongoSessionManager(ctx, db, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authHandler := auth.NewHandler(userRepo, sessions)
	router := api.NewRouter(config.Config{CORSOrigins: []string{"http://localhost"}}, db,
		nil, authHandler, nil, nil, nil, nil, nil, userRepo, nil, sessions)
	server := httptest.NewServer(router)
	defer server.Close()

	first := loginAssetUser(t, server.URL, "alice", "secret")
	second := loginAssetUser(t, server.URL, "alice", "secret")

	call := func(method, path, body string, cookies []*http.Cookie) (int, []byte) {
		var reader io.Reader
		if body != "" {
			reader = bytes.NewBufferString(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, reader)
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

	status, body := call(http.MethodGet, "/api/v1/user/security/sessions", "", first)
	if status != http.StatusOK {
		t.Fatalf("sessions = %d: %s", status, body)
	}
	var listed struct {
		Data struct {
			Items []struct {
				ID      string `json:"id"`
				Current bool   `json:"current"`
				UA      string `json:"ua"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("sessions payload = %s (%v)", body, err)
	}
	if len(listed.Data.Items) != 2 {
		t.Fatalf("sessions = %d", len(listed.Data.Items))
	}
	currentCount, otherDigest := 0, ""
	for _, item := range listed.Data.Items {
		if item.Current {
			currentCount++
		} else {
			otherDigest = item.ID
		}
	}
	if currentCount != 1 || otherDigest == "" {
		t.Fatalf("current markers = %d, other = %q", currentCount, otherDigest)
	}

	if status, _ := call(http.MethodDelete, "/api/v1/user/security/sessions/"+otherDigest, "", first); status != http.StatusNoContent {
		t.Fatalf("revoke session = %d", status)
	}
	if status, _ := call(http.MethodGet, "/api/v1/auth/me", "", second); status != http.StatusUnauthorized {
		t.Fatalf("revoked session still valid = %d", status)
	}
	if status, body := call(http.MethodGet, "/api/v1/user/security/sessions", "", first); status != http.StatusOK {
		t.Fatalf("sessions after revoke = %d: %s", status, body)
	} else if !bytes.Contains(body, []byte(`"current":true`)) || bytes.Count(body, []byte(`"id":`)) != 1 {
		t.Fatalf("sessions after revoke payload = %s", body)
	}

	if status, _ := call(http.MethodPost, "/api/v1/user/security/password",
		`{"current":"wrong","password":"newpass","verifyPassword":"newpass"}`, first); status != http.StatusForbidden {
		t.Fatalf("wrong current password = %d", status)
	}
	if status, _ := call(http.MethodPost, "/api/v1/user/security/password",
		`{"current":"secret","password":"newpass","verifyPassword":"different"}`, first); status != http.StatusBadRequest {
		t.Fatalf("mismatched repeat = %d", status)
	}
	if status, body := call(http.MethodPost, "/api/v1/user/security/password",
		`{"current":"secret","password":"newpass","verifyPassword":"newpass"}`, first); status != http.StatusNoContent {
		t.Fatalf("change password = %d: %s", status, body)
	}
	if status, _ := call(http.MethodGet, "/api/v1/auth/me", "", first); status != http.StatusUnauthorized {
		t.Fatalf("session survived password change = %d", status)
	}
	if _, _, err := loginRaw(server.URL, "alice", "secret"); err == nil {
		t.Fatal("old password still accepted")
	}
	if _, _, err := loginRaw(server.URL, "alice", "newpass"); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
}

// loginRaw returns the status code, cookies and error of a login attempt.
func loginRaw(baseURL, username, password string) (int, []*http.Cookie, error) {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil, fmt.Errorf("login status %d", response.StatusCode)
	}
	return response.StatusCode, response.Cookies(), nil
}
