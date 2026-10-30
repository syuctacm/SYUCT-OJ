package message

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Event is the frame delivered to messagepad clients. The frontend uses the
// embedded message to refresh the dialogue and show the upstream-style popup.
type Event struct {
	Message Message `json:"message"`
}

type messageClient struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub fans messages out to connected messagepad clients, grouped by the
// recipient uid. Like RecordHub it is intentionally in-process; the current
// deployment runs one API instance.
type Hub struct {
	mu       sync.Mutex
	clients  map[int64]map[*messageClient]struct{}
	upgrader websocket.Upgrader
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[int64]map[*messageClient]struct{}),
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

// Publish sends one message to each connected recipient. Duplicate uids are
// collapsed so a malformed multi-recipient message cannot fan out twice.
func (h *Hub) Publish(recipients []int64, item *Message) {
	if h == nil || item == nil || len(recipients) == 0 {
		return
	}
	payload, err := json.Marshal(Event{Message: *item})
	if err != nil {
		return
	}
	seen := make(map[int64]bool, len(recipients))
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, uid := range recipients {
		if uid <= 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		for client := range h.clients[uid] {
			select {
			case client.send <- payload:
			default:
				// A slow tab is caught up by its next messages-list request.
			}
		}
	}
}

// Serve upgrades an authenticated request and keeps the connection in the
// recipient's fan-out group until the client disconnects.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, uid int64) error {
	if h == nil || uid <= 0 {
		return http.ErrNotSupported
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	client := &messageClient{conn: conn, send: make(chan []byte, 32)}
	h.mu.Lock()
	if h.clients[uid] == nil {
		h.clients[uid] = make(map[*messageClient]struct{})
	}
	h.clients[uid][client] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients[uid], client)
		if len(h.clients[uid]) == 0 {
			delete(h.clients, uid)
		}
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
