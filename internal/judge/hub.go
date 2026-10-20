package judge

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu            sync.Mutex
	conns         map[*connection]struct{}
	queue         []any
	languages     map[string]any
	upgrader      websocket.Upgrader
	resultHandler func(map[string]any)
}

type connection struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
	started bool
	hub     *Hub
}

func NewHub(languages map[string]any) *Hub {
	if languages == nil {
		languages = DefaultLanguages()
	}
	return &Hub{
		conns:     make(map[*connection]struct{}),
		languages: languages,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("judge websocket upgrade failed", "error", err)
		return
	}
	conn := &connection{ws: ws, hub: h}
	h.mu.Lock()
	h.conns[conn] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
		_ = ws.Close()
	}()

	if err := conn.writeJSON(map[string]any{"language": h.languages}); err != nil {
		return
	}

	for {
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		if string(message) == "pong" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(message, &payload); err != nil {
			continue
		}
		key, _ := payload["key"].(string)
		switch key {
		case "start":
			h.start(conn)
		case "ping":
			_ = conn.writeText("pong")
		case "next", "end":
			slog.Info("judge result received", "key", key)
			h.mu.Lock()
			resultHandler := h.resultHandler
			h.mu.Unlock()
			if resultHandler != nil {
				resultHandler(payload)
			}
		}
	}
}

func (h *Hub) Enqueue(task any) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.conns {
		if conn.started {
			if err := conn.writeJSON(map[string]any{"task": task}); err == nil {
				return true
			}
		}
	}
	h.queue = append(h.queue, task)
	return false
}

func (h *Hub) SetResultHandler(handler func(map[string]any)) {
	h.mu.Lock()
	h.resultHandler = handler
	h.mu.Unlock()
}

func (h *Hub) HasStarted() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.conns {
		if conn.started {
			return true
		}
	}
	return false
}

func (h *Hub) start(conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conn.started = true
	for len(h.queue) > 0 {
		task := h.queue[0]
		h.queue = h.queue[1:]
		if err := conn.writeJSON(map[string]any{"task": task}); err != nil {
			return
		}
	}
}

func (c *connection) writeJSON(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteJSON(value)
}

func (c *connection) writeText(value string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteMessage(websocket.TextMessage, []byte(value))
}
