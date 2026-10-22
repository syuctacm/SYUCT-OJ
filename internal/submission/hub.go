package submission

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RecordUpdate is the small payload pushed to record-list clients. Code and
// judge detail stay in the per-record API.
type RecordUpdate struct {
	RID     string     `json:"rid"`
	UID     int64      `json:"uid"`
	PID     int64      `json:"pid"`
	Lang    string     `json:"lang"`
	Status  int        `json:"status"`
	Score   int        `json:"score"`
	Time    float64    `json:"time"`
	Memory  int        `json:"memory"`
	JudgeAt *time.Time `json:"judgeAt,omitempty"`
}

type recordClient struct {
	conn *websocket.Conn
	send chan []byte
}

// RecordHub broadcasts record changes to connected record-list clients. It is
// intentionally in-process: one API instance is used in the current deployment.
type RecordHub struct {
	mu       sync.Mutex
	clients  map[*recordClient]struct{}
	upgrader websocket.Upgrader
}

func NewRecordHub() *RecordHub {
	return &RecordHub{
		clients: make(map[*recordClient]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: sameOrigin,
		},
	}
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host == r.Host
}

func (h *RecordHub) Publish(update RecordUpdate) {
	payload, err := json.Marshal(update)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		select {
		case client.send <- payload:
		default:
			// A slow client is caught up on the next page load.
		}
	}
}

func (h *RecordHub) Serve(w http.ResponseWriter, r *http.Request) error {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	client := &recordClient{conn: conn, send: make(chan []byte, 32)}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
		_ = conn.Close()
	}()

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case payload := <-client.send:
				if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
					return
				}
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			close(done)
			return nil
		}
	}
}
