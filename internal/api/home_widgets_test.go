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
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func TestHomeWidgetsIntegration(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI is not set; start local Docker MongoDB to run this integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("syuct_oj_home_widgets_test_%d", time.Now().UnixNano())
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
		"_id": int64(7), "uname": "alice", "unameLower": "alice",
		"mail": "alice@example.com", "mailLower": "alice@example.com",
		"salt": "salt", "hash": hash, "hashType": "hydro", "priv": int64(16842756),
	}); err != nil {
		t.Fatal(err)
	}
	location := time.FixedZone("CST", 8*60*60)
	now := time.Now().In(location)
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	past := now.AddDate(0, 0, -2).Format("2006-01-02")

	userRepo := user.NewRepository(db)
	sessions := auth.NewSessionManager(time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	userHandler := handler.NewUserHandler(userRepo, sessions, nil)
	countdownHandler := handler.NewCountdownHandler("倒计时", 2, []config.CountdownDate{
		{Name: "过期事件", Date: past},
		{Name: "今天", Date: today},
		{Name: "明天", Date: tomorrow},
	})
	router := gin.New()
	authHandler.RegisterRoutes(router.Group("/"))
	userHandler.RegisterRoutes(router.Group("/"))
	countdownHandler.RegisterRoutes(router.Group("/"))
	server := httptest.NewServer(router)
	defer server.Close()

	cookies := loginAssetUser(t, server.URL, "alice", "secret")
	status, body := verifyRequest(t, http.MethodGet, server.URL+"/api/v1/user/checkin", nil, cookies)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"checkedIn":false`)) {
		t.Fatalf("initial checkin = %d: %s", status, body)
	}
	status, body = verifyRequest(t, http.MethodPost, server.URL+"/api/v1/user/checkin", nil, cookies)
	if status != http.StatusOK {
		t.Fatalf("checkin = %d: %s", status, body)
	}
	var first struct {
		Data user.Checkin `json:"data"`
	}
	if err := json.Unmarshal(body, &first); err != nil {
		t.Fatal(err)
	}
	if !first.Data.CheckedIn || first.Data.CountNow != 1 || first.Data.CountAll != 1 || first.Data.Luck < 0 || first.Data.Luck > 6 {
		t.Fatalf("first checkin = %+v", first.Data)
	}
	status, body = verifyRequest(t, http.MethodPost, server.URL+"/api/v1/user/checkin", nil, cookies)
	var duplicate struct {
		Data user.Checkin `json:"data"`
	}
	if err := json.Unmarshal(body, &duplicate); err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || duplicate.Data.CountNow != 1 || duplicate.Data.CountAll != 1 || duplicate.Data.Luck != first.Data.Luck {
		t.Fatalf("duplicate checkin = %d %+v", status, duplicate.Data)
	}

	if _, err := db.Collection("user").UpdateOne(ctx, bson.M{"_id": int64(7)}, bson.M{"$set": bson.M{
		"checkin_time": yesterday, "checkin_luck": int64(2), "checkin_cnt_now": int64(4), "checkin_cnt_all": int64(10),
	}}); err != nil {
		t.Fatal(err)
	}
	status, body = verifyRequest(t, http.MethodPost, server.URL+"/api/v1/user/checkin", nil, cookies)
	var streak struct {
		Data user.Checkin `json:"data"`
	}
	if err := json.Unmarshal(body, &streak); err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || streak.Data.CountNow != 5 || streak.Data.CountAll != 11 {
		t.Fatalf("streak checkin = %d %+v", status, streak.Data)
	}

	status, body = verifyRequest(t, http.MethodGet, server.URL+"/api/v1/home/countdown", nil, nil)
	var countdown struct {
		Data struct {
			Title string `json:"title"`
			Items []struct {
				Name string `json:"name"`
				Days int    `json:"days"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &countdown); err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || countdown.Data.Title != "倒计时" || len(countdown.Data.Items) != 2 ||
		countdown.Data.Items[0].Name != "今天" || countdown.Data.Items[0].Days != 0 ||
		countdown.Data.Items[1].Name != "明天" || countdown.Data.Items[1].Days != 1 {
		t.Fatalf("countdown = %d %+v", status, countdown.Data)
	}
}

func verifyRequest(t *testing.T, method, url string, body []byte, cookies []*http.Cookie) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
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
