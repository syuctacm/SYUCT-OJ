package judge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHubBroadcastsLanguageUpdates(t *testing.T) {
	_, hub, sessions, router := newTestHandler(t)
	server := httptest.NewServer(router)
	defer server.Close()

	token, err := sessions.Issue()
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/judge/conn"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := client.ReadMessage(); err != nil {
		t.Fatalf("read initial language message: %v", err)
	}

	languages, err := ParseLanguages("rust:\n  display: Rust Test\n  compile: rustc -o foo foo.rs\n  code_file: foo.rs\n  execute: ./foo\n")
	if err != nil {
		t.Fatal(err)
	}
	hub.SetLanguages(languages)
	_, message, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read language update: %v", err)
	}
	if !bytes.Contains(message, []byte("language")) || !bytes.Contains(message, []byte("Rust Test")) {
		t.Fatalf("unexpected language update: %s", message)
	}
	var payload struct {
		Language map[string]any `json:"language"`
	}
	if err := json.Unmarshal(message, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload.Language["rust"]; !ok {
		t.Fatalf("updated language table = %#v", payload.Language)
	}
}
