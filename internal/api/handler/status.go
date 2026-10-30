package handler

import (
	"net/http"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/judge"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type StatusHandler struct {
	db       *mongo.Database
	requests atomic.Int64
}

func NewStatusHandler(db *mongo.Database) *StatusHandler {
	return &StatusHandler{db: db}
}

func (h *StatusHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/status", h.get)
}

type statusStat struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	IsOnline bool              `json:"isOnline"`
	UpdateAt time.Time         `json:"updateAt"`
	OSInfo   map[string]string `json:"osinfo"`
	CPU      map[string]string `json:"cpu"`
	Memory   map[string]int64  `json:"memory"`
	ReqCount int64             `json:"reqCount"`
}

func (h *StatusHandler) get(c *gin.Context) {
	h.requests.Add(1)
	started := time.Now().UTC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	online := true
	if h.db != nil {
		if err := h.db.Client().Ping(c.Request.Context(), nil); err != nil {
			online = false
		}
	}
	status := "Online"
	if !online {
		status = "Database unavailable"
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{
			"stats": []statusStat{{
				ID: "api", Status: status, IsOnline: online, UpdateAt: started,
				OSInfo: map[string]string{
					"distro": "Go", "release": runtime.Version(), "codename": "", "arch": runtime.GOARCH,
				},
				CPU: map[string]string{
					"manufacturer": "SyUCT OJ", "brand": "API runtime", "speed": strconv.Itoa(runtime.NumCPU()),
				},
				Memory:   map[string]int64{"used": int64(mem.Alloc), "total": int64(mem.Sys)},
				ReqCount: h.requests.Load(),
			}},
			"languages": statusLanguages(),
			"compilers": []any{},
		},
	})
}

func statusLanguages() map[string]string {
	result := make(map[string]string)
	for key, raw := range judge.DefaultLanguages() {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if hidden, _ := entry["hidden"].(bool); hidden {
			continue
		}
		if remote, _ := entry["remote"].(bool); remote {
			continue
		}
		display, _ := entry["display"].(string)
		if display == "" {
			display = key
		}
		command, _ := entry["compile"].(string)
		if command == "" {
			command, _ = entry["execute"].(string)
		}
		result[display+" ("+key+")"] = command
	}
	return result
}
