package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type HealthHandler struct {
	db *mongo.Database
}

func NewHealthHandler(db *mongo.Database) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Client().Ping(ctx, nil); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "degraded",
			"service":   "syuct-oj-api",
			"database":  "unavailable",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "syuct-oj-api",
		"database":  "up",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
