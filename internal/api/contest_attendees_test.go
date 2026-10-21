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
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestContestAttendeesIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("syuct_oj_contest_attendees_test_%d", time.Now().UnixNano())
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
		"title": "报名测试", "rule": "acm", "owner": int64(1), "attend": int64(0),
		"beginAt": now.Add(time.Hour), "endAt": now.Add(3 * time.Hour),
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

	type attendeePage struct {
		Data struct {
			Items []struct {
				UID    int64 `json:"uid"`
				Unrank bool  `json:"unrank"`
			} `json:"items"`
			Users []struct {
				ID int64 `json:"id"`
			} `json:"users"`
		} `json:"data"`
	}
	list := func(cookies []*http.Cookie) (int, attendeePage) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/contests/"+contestID.Hex()+"/attendees", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var page attendeePage
		if resp.StatusCode == http.StatusOK {
			if err := decodeJSON(resp.Body, &page); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, page
	}

	if status, _ := list(aliceCookies); status != http.StatusForbidden {
		t.Fatalf("non-editor list = %d", status)
	}

	body := bytes.NewBufferString(`{"uids":[7,8]}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests/"+contestID.Hex()+"/attendees", body)
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("add attendees = %d", resp.StatusCode)
	}

	status, page := list(adminCookies)
	if status != http.StatusOK || len(page.Data.Items) != 2 || len(page.Data.Users) != 2 {
		t.Fatalf("attendee list = %d %+v", status, page.Data)
	}

	req, _ = http.NewRequest(http.MethodPatch, server.URL+"/api/v1/contests/"+contestID.Hex()+"/attendees/8", bytes.NewBufferString(`{"unrank":true}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("set unrank = %d", resp.StatusCode)
	}
	_, page = list(adminCookies)
	found := false
	for _, item := range page.Data.Items {
		if item.UID == 8 && item.Unrank {
			found = true
		}
	}
	if !found {
		t.Fatalf("unrank not persisted: %+v", page.Data.Items)
	}

	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/contests/"+contestID.Hex()+"/attendees/8", nil)
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel registration = %d", resp.StatusCode)
	}
	_, page = list(adminCookies)
	if len(page.Data.Items) != 1 || page.Data.Items[0].UID != 7 {
		t.Fatalf("attendees after cancel = %+v", page.Data.Items)
	}
	var doc struct {
		Attend int64 `bson:"attend"`
	}
	if err := db.Collection("document").FindOne(ctx, bson.M{"_id": contestID}).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Attend != 1 {
		t.Fatalf("attend counter = %d", doc.Attend)
	}

	// Flexible-duration attendees can be resumed while the contest is live.
	if _, err := db.Collection("document.status").UpdateOne(ctx,
		bson.M{"docId": contestID, "uid": int64(7)},
		bson.M{"$set": bson.M{"endAt": now.Add(30 * time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests/"+contestID.Hex()+"/attendees/7/resume", nil)
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("resume = %d", resp.StatusCode)
	}
	var resumed struct {
		EndAt *time.Time `bson:"endAt"`
	}
	if err := db.Collection("document.status").FindOne(ctx, bson.M{"docId": contestID, "uid": int64(7)}).Decode(&resumed); err != nil {
		t.Fatal(err)
	}
	if resumed.EndAt != nil {
		t.Fatalf("endAt should be cleared: %v", *resumed.EndAt)
	}
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/api/v1/contests/"+contestID.Hex()+"/attendees/7/resume", nil)
	for _, cookie := range aliceCookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-editor resume = %d", resp.StatusCode)
	}

	// Registrations cannot be cancelled once the contest started.
	if _, err := db.Collection("document").UpdateOne(ctx, bson.M{"_id": contestID}, bson.M{"$set": bson.M{"beginAt": now.Add(-time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/contests/"+contestID.Hex()+"/attendees/7", nil)
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cancel after start = %d", resp.StatusCode)
	}
}
