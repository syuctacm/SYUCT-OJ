package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/discussion"
)

type DiscussionHandler struct {
	repo     *discussion.Repository
	sessions *auth.SessionManager
}

func NewDiscussionHandler(repo *discussion.Repository, sessions *auth.SessionManager) *DiscussionHandler {
	return &DiscussionHandler{repo: repo, sessions: sessions}
}

func (h *DiscussionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/discussions", h.list)
	router.GET("/api/v1/discussions/:did", h.get)
	router.GET("/api/v1/discussions/:did/replies", h.replies)
	router.POST("/api/v1/discussions/:did/replies", h.createReply)
}

func (h *DiscussionHandler) list(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list discussions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
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
	session, ok := h.sessions.Get(auth.Token(c))
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
	reply, err := h.repo.AddReply(c.Request.Context(), item.ID, session.UserID, request.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create reply"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": reply})
}
