package handler

import (
	"context"
	"fmt"
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
	router.PATCH("/api/v1/admin/users/:uid/priv", h.updateUserPriv)
	router.PATCH("/api/v1/admin/users/:uid/ban", h.setUserBanned)
	router.POST("/api/v1/admin/users/import", h.importUsers)
	router.GET("/api/v1/admin/settings", h.settings)
	router.PUT("/api/v1/admin/settings", h.updateSettings)
	router.GET("/api/v1/admin/ip-login", h.listIPLogins)
	router.PUT("/api/v1/admin/ip-login/:ip", h.setIPLogin)
	router.DELETE("/api/v1/admin/ip-login/:ip", h.deleteIPLogin)
}

type adminIPLogin struct {
	IP       string `json:"ip"`
	UID      int64  `json:"uid"`
	Username string `json:"username,omitempty"`
}

func (h *AdminHandler) listIPLogins(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	items, err := h.users.ListIPLogins(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list IP logins"})
		return
	}
	uids := make([]int64, 0, len(items))
	for _, item := range items {
		uids = append(uids, item.UID)
	}
	accounts, err := h.users.FindByIDs(c.Request.Context(), uids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load IP login users"})
		return
	}
	usernames := make(map[int64]string, len(accounts))
	for _, account := range accounts {
		usernames[account.ID] = account.Uname
	}
	result := make([]adminIPLogin, 0, len(items))
	for _, item := range items {
		result = append(result, adminIPLogin{IP: item.IP, UID: item.UID, Username: usernames[item.UID]})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": result}})
}

func (h *AdminHandler) setIPLogin(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	ip := user.NormalizeIP(c.Param("ip"))
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
		return
	}
	var request struct {
		UID int64 `json:"uid"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.UID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
		return
	}
	if _, err := h.users.FindByID(c.Request.Context(), request.UID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err := h.users.SetIPLogin(c.Request.Context(), ip, request.UID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save IP login"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"ip": ip, "uid": request.UID}})
}

func (h *AdminHandler) deleteIPLogin(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	ip := user.NormalizeIP(c.Param("ip"))
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
		return
	}
	if err := h.users.DeleteIPLogin(c.Request.Context(), ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete IP login"})
		return
	}
	c.Status(http.StatusNoContent)
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
	Banned      bool      `bson:"banned,omitempty" json:"banned"`
	Avatar      string    `bson:"avatar" json:"avatar,omitempty"`
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
	if limit > 1000 {
		limit = 1000
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
	sort := bson.D{{Key: "regat", Value: -1}, {Key: "_id", Value: -1}}
	if c.Query("sort") == "id" {
		sort = bson.D{{Key: "_id", Value: 1}}
	}
	cursor, err := coll.Find(c.Request.Context(), filter, options.Find().
		SetProjection(bson.M{"uname": 1, "displayName": 1, "avatar": 1, "mail": 1, "studentId": 1, "priv": 1, "banned": 1, "regat": 1, "loginat": 1}).
		SetSort(sort).
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

func (h *AdminHandler) settings(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	values, err := readSettings(c.Request.Context(), h.db, PublicSettingKeys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": values})
}

func (h *AdminHandler) updateSettings(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	var request struct {
		Bulletin string `json:"bulletin"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(request.Bulletin) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bulletin is too long"})
		return
	}
	if _, err := h.db.Collection("system").UpdateOne(c.Request.Context(), bson.M{"_id": "bulletin"},
		bson.M{"$set": bson.M{"value": request.Bulletin}}, options.UpdateOne().SetUpsert(true)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"bulletin": request.Bulletin}})
}

func (h *AdminHandler) updateUserPriv(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	session, _ := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if uid == session.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot change your own privileges"})
		return
	}
	var request struct {
		Priv int64 `json:"priv"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Priv < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid privilege value"})
		return
	}
	// Mirrors Hydro's CannotEditSuperAdminError: PRIV_ALL accounts are protected.
	var target struct {
		Priv int64 `bson:"priv"`
	}
	if err := h.db.Collection("user").FindOne(c.Request.Context(), bson.M{"_id": uid}).Decode(&target); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if target.Priv < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot edit a super administrator"})
		return
	}
	result, err := h.db.Collection("user").UpdateOne(c.Request.Context(), bson.M{"_id": uid}, bson.M{"$set": bson.M{"priv": request.Priv}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update user"})
		return
	}
	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"id": uid, "priv": request.Priv}})
}

func (h *AdminHandler) setUserBanned(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	session, _ := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if uid == session.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot ban yourself"})
		return
	}
	target, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if uid == 1 || target.Priv < 0 || target.Priv&user.PrivEditSystem != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot ban an administrator"})
		return
	}
	var request struct {
		Banned bool `json:"banned"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	priv, err := h.users.SetBanned(c.Request.Context(), uid, request.Banned)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update user"})
		return
	}
	if request.Banned {
		if err := h.sessions.RevokeUser(c.Request.Context(), uid); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to revoke user sessions"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"id": uid, "priv": priv, "banned": request.Banned,
	}})
}

// importUsers creates accounts in bulk from the admin import textarea.
func (h *AdminHandler) importUsers(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	var request struct {
		Users string `json:"users"`
		Draft bool   `json:"draft"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Users) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "users is required"})
		return
	}
	parsed, messages := user.ParseImportUsers(request.Users)
	if request.Draft {
		// Upstream also reports rows that already exist while previewing.
		for _, item := range parsed {
			if _, err := h.users.FindByMail(c.Request.Context(), item.Email); err == nil {
				messages = append(messages, fmt.Sprintf("%s：邮箱已注册", item.Email))
				continue
			}
			if _, err := h.users.FindByUsername(c.Request.Context(), item.Username); err == nil {
				messages = append(messages, fmt.Sprintf("%s：用户名已被占用", item.Username))
			}
		}
		messages = append(messages, fmt.Sprintf("找到 %d 个用户", len(parsed)))
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
			"users": parsed, "messages": messages, "created": 0,
		}})
		return
	}
	uids, createMessages, err := h.users.ImportUsers(c.Request.Context(), parsed, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to import users"})
		return
	}
	messages = append(messages, createMessages...)
	messages = append(messages, fmt.Sprintf("成功创建 %d 个用户", len(uids)))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"users": parsed, "messages": messages, "created": len(uids),
	}})
}

func (h *AdminHandler) requireAdmin(c *gin.Context) bool {
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
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
