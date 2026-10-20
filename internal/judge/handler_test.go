package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
)

func newTestHandler(t *testing.T) (*Handler, *Hub, *auth.SessionManager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sessions := auth.NewSessionManager(time.Hour)
	store := NewFileStore(t.TempDir())
	if err := store.Ensure(); err != nil {
		t.Fatalf("ensure store: %v", err)
	}
	hub := NewHub(DefaultLanguages())
	handler := NewHandler("judge", "secret", sessions, store, hub)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/"))
	return handler, hub, sessions, router
}

func loginCookies(t *testing.T, router *gin.Engine) []*http.Cookie {
	t.Helper()
	form := url.Values{
		"uname":    {"judge"},
		"password": {"secret"},
	}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a cookie")
	}
	return cookies
}

func doJSON(t *testing.T, router *gin.Engine, method, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestJudgeHTTPContract(t *testing.T) {
	handler, _, sessions, router := newTestHandler(t)
	cookies := loginCookies(t, router)

	if recorder := doJSON(t, router, http.MethodGet, "/judge/files", nil, cookies); recorder.Code != http.StatusOK {
		t.Fatalf("files check status = %d", recorder.Code)
	}

	if err := handler.store.SaveSubmission("rid-1", strings.NewReader("int main() { return 0; }")); err != nil {
		t.Fatalf("save submission: %v", err)
	}
	recorder := doJSON(t, router, http.MethodPost, "/judge/files", map[string]string{"id": "rid-1"}, cookies)
	if recorder.Code != http.StatusOK {
		t.Fatalf("files link status = %d", recorder.Code)
	}
	var linkResponse struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &linkResponse); err != nil {
		t.Fatalf("decode link response: %v", err)
	}
	if linkResponse.URL == "" {
		t.Fatal("empty submission file URL")
	}
	codeRequest := httptest.NewRequest(http.MethodGet, linkResponse.URL, nil)
	for _, cookie := range cookies {
		codeRequest.AddCookie(cookie)
	}
	codeRecorder := httptest.NewRecorder()
	router.ServeHTTP(codeRecorder, codeRequest)
	if codeRecorder.Code != http.StatusOK || !strings.Contains(codeRecorder.Body.String(), "int main") {
		t.Fatalf("submission file response = %d %q", codeRecorder.Code, codeRecorder.Body.String())
	}

	if err := handler.store.SaveProblemFile("system", "1001", "1.in", strings.NewReader("42\n")); err != nil {
		t.Fatalf("save problem file: %v", err)
	}
	recorder = doJSON(t, router, http.MethodPost, "/d/system/judge/files", map[string]any{
		"pid":   1001,
		"files": []string{"1.in"},
	}, cookies)
	if recorder.Code != http.StatusOK {
		t.Fatalf("problem links status = %d", recorder.Code)
	}
	var problemResponse struct {
		Links map[string]string `json:"links"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &problemResponse); err != nil {
		t.Fatalf("decode problem links: %v", err)
	}
	if problemResponse.Links["1.in"] == "" {
		t.Fatal("missing problem file link")
	}
	if sessions.Validate("") {
		t.Fatal("empty session must be invalid")
	}
}

func TestJudgeWebSocketContract(t *testing.T) {
	_, hub, sessions, router := newTestHandler(t)
	server := httptest.NewServer(router)
	defer server.Close()

	token, err := sessions.Issue()
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/judge/conn"
	header := http.Header{"Authorization": []string{"Bearer " + token}}
	client, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))

	_, languageMessage, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read language message: %v", err)
	}
	if !bytes.Contains(languageMessage, []byte("language")) {
		t.Fatalf("unexpected language message: %s", languageMessage)
	}

	if err := client.WriteJSON(map[string]string{"key": "start"}); err != nil {
		t.Fatalf("send start: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for !hub.HasStarted() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !hub.HasStarted() {
		t.Fatal("judge did not enter started state")
	}
	if !hub.Enqueue(Task{
		Rid:      "rid-ws",
		Lang:     "cpp",
		DomainID: "system",
		Type:     "judge",
	}) {
		t.Fatal("unable to enqueue task")
	}

	_, taskMessage, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read task message: %v", err)
	}
	if !bytes.Contains(taskMessage, []byte(`"task"`)) || !bytes.Contains(taskMessage, []byte(`"rid-ws"`)) {
		t.Fatalf("unexpected task message: %s", taskMessage)
	}
	if active := hub.ActiveRIDs(); len(active) != 1 || active[0] != "rid-ws" {
		t.Fatalf("active judge tasks: %v", active)
	}
	hub.SetResultHandler(func(map[string]any) error {
		return errors.New("persistence failed")
	})
	if err := client.WriteJSON(map[string]string{"key": "end", "rid": "rid-ws"}); err != nil {
		t.Fatalf("send end: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if active := hub.ActiveRIDs(); len(active) != 1 {
		t.Fatalf("task was released after persistence failure: %v", active)
	}
	hub.SetResultHandler(nil)
	if err := client.WriteJSON(map[string]string{"key": "end", "rid": "rid-ws"}); err != nil {
		t.Fatalf("send end retry: %v", err)
	}
	deadline = time.Now().Add(time.Second)
	for len(hub.ActiveRIDs()) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if active := hub.ActiveRIDs(); len(active) != 0 {
		t.Fatalf("completed task remains active: %v", active)
	}
}

func TestJudgeWebSocketPingContract(t *testing.T) {
	_, _, sessions, router := newTestHandler(t)
	server := httptest.NewServer(router)
	defer server.Close()

	token, err := sessions.Issue()
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/judge/conn"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer " + token},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))

	if _, _, err := client.ReadMessage(); err != nil {
		t.Fatalf("read language message: %v", err)
	}

	if err := client.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("send text ping: %v", err)
	}
	_, pong, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read text pong: %v", err)
	}
	if string(pong) != "pong" {
		t.Fatalf("unexpected text heartbeat response: %q", pong)
	}

	if err := client.WriteJSON(map[string]string{"key": "ping"}); err != nil {
		t.Fatalf("send JSON ping: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if _, message, err := client.ReadMessage(); err == nil {
		t.Fatalf("JSON ping must not produce a websocket response: %s", message)
	}
}

func TestDefaultLanguagesMatchHydroContract(t *testing.T) {
	languages := DefaultLanguages()
	for name, raw := range languages {
		lang, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("language %s is %T", name, raw)
		}
		if lang["key"] != name {
			t.Fatalf("language %s key = %v", name, lang["key"])
		}
		for _, key := range []string{"display", "compile", "execute", "code_file", "target"} {
			if value, ok := lang[key].(string); !ok || value == "" {
				t.Fatalf("language %s missing %s: %v", name, key, lang[key])
			}
		}
	}
}

func TestTaskRIDFromBSON(t *testing.T) {
	rid := bson.NewObjectID()
	if got := taskRID(bson.M{"rid": rid}); got != rid.Hex() {
		t.Fatalf("taskRID(bson.M) = %q, want %q", got, rid.Hex())
	}
}
