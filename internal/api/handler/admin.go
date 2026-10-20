package handler

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

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
	router.GET("/api/v1/admin/users", h.userList)
}

func (h *AdminHandler) stats(c *gin.Context) {
	if !h.requireAdmin(c) {
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

type AdminUser struct {
	ID          int64     `bson:"_id" json:"id"`
	Username    string    `bson:"uname" json:"username"`
	DisplayName string    `bson:"displayName" json:"displayName,omitempty"`
	Email       string    `bson:"mail" json:"email"`
	StudentID   string    `bson:"studentId" json:"studentId,omitempty"`
	Priv        int64     `bson:"priv" json:"priv"`
	RegAt       time.Time `bson:"regat" json:"regAt"`
	LoginAt     time.Time `bson:"loginat" json:"loginAt"`
}

func (h *AdminHandler) userList(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	filter := bson.M{}
	if query := strings.TrimSpace(c.Query("q")); query != "" {
		pattern := regexp.QuoteMeta(query)
		filter["$or"] = []bson.M{
			{"unameLower": bson.M{"$regex": pattern, "$options": "i"}},
			{"mailLower": bson.M{"$regex": pattern, "$options": "i"}},
			{"displayName": bson.M{"$regex": pattern, "$options": "i"}},
			{"studentId": bson.M{"$regex": pattern, "$options": "i"}},
		}
	}
	coll := h.db.Collection("user")
	total, err := coll.CountDocuments(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to count users"})
		return
	}
	cursor, err := coll.Find(c.Request.Context(), filter, options.Find().
		SetProjection(bson.M{"uname": 1, "displayName": 1, "mail": 1, "studentId": 1, "priv": 1, "regat": 1, "loginat": 1}).
		SetSort(bson.D{{Key: "regat", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page-1)*limit)).
		SetLimit(int64(limit)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list users"})
		return
	}
	defer cursor.Close(c.Request.Context())
	items := make([]AdminUser, 0, limit)
	if err := cursor.All(c.Request.Context(), &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to decode users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": total, "page": page, "limit": limit}})
}

func (h *AdminHandler) requireAdmin(c *gin.Context) bool {
	session, ok := h.sessions.Get(auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return false
	}
	u, err := h.users.FindByID(c.Request.Context(), session.UserID)
	if err != nil || u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return false
	}
	return true
}

func (h *AdminHandler) count(ctx context.Context, collection string, filter bson.M) int64 {
	n, err := h.db.Collection(collection).CountDocuments(ctx, filter)
	if err != nil {
		return -1
	}
	return n
}
