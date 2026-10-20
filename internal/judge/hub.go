package judge

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Hub struct {
	mu            sync.Mutex
	conns         map[*connection]struct{}
	languages     map[string]any
	upgrader      websocket.Upgrader
	resultHandler func(map[string]any) error
}

type connection struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
	started bool
	tasks   map[string]struct{}
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
	conn := &connection{ws: ws, hub: h, tasks: make(map[string]struct{})}
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
		if string(message) == "ping" {
			_ = conn.writeText("pong")
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
		case "next", "end":
			slog.Info("judge result received", "key", key)
			h.mu.Lock()
			resultHandler := h.resultHandler
			h.mu.Unlock()
			if resultHandler != nil {
				if err := resultHandler(payload); err != nil {
					slog.Warn("unable to process judge result", "key", key, "error", err)
					continue
				}
			}
			if key == "end" {
				if rid, ok := payload["rid"].(string); ok {
					h.mu.Lock()
					for active := range h.conns {
						delete(active.tasks, rid)
					}
					h.mu.Unlock()
				}
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
				if rid := taskRID(task); rid != "" {
					conn.tasks[rid] = struct{}{}
				}
				return true
			}
		}
	}
	return false
}

func (h *Hub) SetResultHandler(handler func(map[string]any) error) {
	h.mu.Lock()
	h.resultHandler = handler
	h.mu.Unlock()
}

func (h *Hub) ActiveRIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	seen := map[string]struct{}{}
	for conn := range h.conns {
		if !conn.started {
			continue
		}
		for rid := range conn.tasks {
			seen[rid] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for rid := range seen {
		result = append(result, rid)
	}
	return result
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

func taskRID(task any) string {
	switch value := task.(type) {
	case Task:
		return value.Rid
	case map[string]any:
		return objectIDString(value["rid"])
	case bson.M:
		return objectIDString(value["rid"])
	default:
		return ""
	}
}

func objectIDString(value any) string {
	switch id := value.(type) {
	case bson.ObjectID:
		return id.Hex()
	case string:
		return id
	default:
		return ""
	}
}
