package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type DiscussionHandler struct {
	repo     *discussion.Repository
	sessions *auth.SessionManager
	users    *user.Repository
}

func NewDiscussionHandler(repo *discussion.Repository, sessions *auth.SessionManager, users *user.Repository) *DiscussionHandler {
	return &DiscussionHandler{repo: repo, sessions: sessions, users: users}
}

func (h *DiscussionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/discussions", h.list)
	router.GET("/api/v1/discussion-nodes", h.listNodes)
	router.POST("/api/v1/discussions", h.create)
	router.PATCH("/api/v1/discussions/:did", h.update)
	router.DELETE("/api/v1/discussions/:did", h.delete)
	router.PATCH("/api/v1/discussions/:did/lock", h.setLock)
	router.GET("/api/v1/discussions/:did", h.get)
	router.GET("/api/v1/discussions/:did/replies", h.replies)
	router.POST("/api/v1/discussions/:did/replies", h.createReply)
	router.PATCH("/api/v1/discussions/:did/replies/:rid", h.updateReply)
	router.DELETE("/api/v1/discussions/:did/replies/:rid", h.deleteReply)
}

func (h *DiscussionHandler) list(c *gin.Context) {
	parentType, _ := strconv.ParseInt(c.Query("parentType"), 10, 64)
	page, limit := parsePaging(c)
	items, total, err := h.repo.ListFiltered(c.Request.Context(), parentType, c.Query("parentId"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list discussions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": total, "page": page, "limit": limit},
	})
}

func (h *DiscussionHandler) listNodes(c *gin.Context) {
	items, err := h.repo.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list discussion nodes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *DiscussionHandler) create(c *gin.Context) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		NodeID    string `json:"nodeId"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		Pin       bool   `json:"pin"`
		Highlight bool   `json:"highlight"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.NodeID = strings.TrimSpace(request.NodeID)
	request.Title = strings.TrimSpace(request.Title)
	if request.NodeID == "" || request.Title == "" || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node, title and content are required"})
		return
	}
	if len(request.Title) > discussion.MaxTitleLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is too long"})
		return
	}
	if len(request.Content) > discussion.MaxContentLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is too long"})
		return
	}
	if (request.Pin || request.Highlight) && !h.isAdmin(c.Request.Context(), session.UserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}
	node, err := h.repo.GetNode(c.Request.Context(), request.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "discussion node not found"})
		return
	}
	item, err := h.repo.Create(c.Request.Context(), node, session.UserID, request.Title, request.Content, request.Pin, request.Highlight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create discussion"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *DiscussionHandler) update(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	admin := h.isAdmin(c.Request.Context(), uid)
	if item.Owner != uid && !admin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var request discussion.EditInput
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and content are required"})
		return
	}
	if len(request.Title) > discussion.MaxTitleLength || len(request.Content) > discussion.MaxContentLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title or content is too long"})
		return
	}
	if !admin && (request.Pin != item.Pin || request.Highlight != item.Highlight) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}
	updated, err := h.repo.Update(c.Request.Context(), c.Param("did"), request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update discussion"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": updated})
}

func (h *DiscussionHandler) delete(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	if item.Owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), c.Param("did")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete discussion"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *DiscussionHandler) setLock(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}
	var request struct {
		Lock bool `json:"lock"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := h.repo.SetLock(c.Request.Context(), c.Param("did"), request.Lock)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *DiscussionHandler) requireUser(c *gin.Context) (int64, bool) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	return session.UserID, true
}

func (h *DiscussionHandler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *DiscussionHandler) replies(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	items, err := h.repo.ListReplies(c.Request.Context(), item.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list replies"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *DiscussionHandler) createReply(c *gin.Context) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	if item.Lock {
		c.JSON(http.StatusConflict, gin.H{"error": "discussion is locked"})
		return
	}
	var request struct {
		Content string `json:"content"`
		ReplyTo string `json:"replyTo"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is required"})
		return
	}
	if len(request.Content) > discussion.MaxReplyLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is too long"})
		return
	}
	if request.ReplyTo != "" {
		replyID, err := bson.ObjectIDFromHex(request.ReplyTo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid replyTo"})
			return
		}
		if _, tail, err := h.repo.AddTailReply(c.Request.Context(), item.ID, replyID, session.UserID, request.Content); err != nil {
			if err == discussion.ErrNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create reply"})
			return
		} else {
			c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": tail})
			return
		}
	}
	reply, err := h.repo.AddReply(c.Request.Context(), item.ID, session.UserID, request.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create reply"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": reply})
}

func (h *DiscussionHandler) updateReply(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, replyID, ok := h.replyContext(c)
	if !ok {
		return
	}
	owner, err := h.repo.ReplyOwner(c.Request.Context(), item.ID, replyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var request struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is required"})
		return
	}
	if len(request.Content) > discussion.MaxReplyLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply content is too long"})
		return
	}
	reply, tail, err := h.repo.UpdateReply(c.Request.Context(), item.ID, replyID, request.Content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if tail != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": tail})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": reply})
}

func (h *DiscussionHandler) deleteReply(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, replyID, ok := h.replyContext(c)
	if !ok {
		return
	}
	owner, err := h.repo.ReplyOwner(c.Request.Context(), item.ID, replyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if owner != uid && item.Owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.repo.DeleteReply(c.Request.Context(), item.ID, replyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete reply"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *DiscussionHandler) replyContext(c *gin.Context) (*discussion.Discussion, bson.ObjectID, bool) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return nil, bson.NilObjectID, false
	}
	replyID, err := bson.ObjectIDFromHex(c.Param("rid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reply id"})
		return nil, bson.NilObjectID, false
	}
	return item, replyID, true
}

func (h *DiscussionHandler) isAdmin(ctx context.Context, uid int64) bool {
	if h.users == nil {
		return false
	}
	u, err := h.users.FindByID(ctx, uid)
	return err == nil && u.Priv&1 != 0
}
