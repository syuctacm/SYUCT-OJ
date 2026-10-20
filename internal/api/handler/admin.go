package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type AdminHandler struct {
	db       *mongo.Database
	users    *user.Repository
	sessions *auth.SessionManager
}

func NewAdminHandler(db *mongo.Database, users *user.Repository, sessions *auth.SessionManager) *AdminHandler {
	return &AdminHandler{db: db, users: users, sessions: sessions}
}

func (h *AdminHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/admin/stats", h.stats)
}

func (h *AdminHandler) stats(c *gin.Context) {
	session, ok := h.sessions.Get(auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	u, err := h.users.FindByID(c.Request.Context(), session.UserID)
	if err != nil || u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}
	ctx := c.Request.Context()
	startOfDay := bson.NewObjectIDFromTimestamp(time.Now().UTC().Truncate(24 * time.Hour))
	counts := gin.H{
		"users":             h.count(ctx, "user", bson.M{}),
		"problems":          h.count(ctx, "document", bson.M{"domainId": "system", "docType": 10}),
		"contests":          h.count(ctx, "document", bson.M{"domainId": "system", "docType": 30, "rule": bson.M{"$ne": "homework"}}),
		"homeworks":         h.count(ctx, "document", bson.M{"domainId": "system", "docType": 30, "rule": "homework"}),
		"trainings":         h.count(ctx, "document", bson.M{"domainId": "system", "docType": 40}),
		"discussions":       h.count(ctx, "document", bson.M{"domainId": "system", "docType": 21}),
		"records":           h.count(ctx, "record", bson.M{}),
		"submissions_today": h.count(ctx, "record", bson.M{"_id": bson.M{"$gte": startOfDay}}),
		"queued_tasks":      h.count(ctx, "task", bson.M{}),
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": counts})
}

func (h *AdminHandler) count(ctx context.Context, collection string, filter bson.M) int64 {
	n, err := h.db.Collection(collection).CountDocuments(ctx, filter)
	if err != nil {
		return -1
	}
	return n
}
