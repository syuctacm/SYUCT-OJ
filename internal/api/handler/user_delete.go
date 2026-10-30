package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type UserDeleteHandler struct {
	users    *user.Repository
	sessions *auth.SessionManager
}

func NewUserDeleteHandler(users *user.Repository, sessions *auth.SessionManager) *UserDeleteHandler {
	return &UserDeleteHandler{users: users, sessions: sessions}
}

func (h *UserDeleteHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/user/delete", h.status)
	router.POST("/api/v1/user/delete", h.request)
	router.POST("/api/v1/user/delete/cancel", h.cancel)
}

func (h *UserDeleteHandler) requireUser(c *gin.Context) (int64, bool) {
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

func (h *UserDeleteHandler) status(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	at, err := h.users.DeletionAt(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load account status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"deleteAt": at}})
}

func (h *UserDeleteHandler) request(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}
	account, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil || user.VerifyPassword(account, request.Password) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}
	at, err := h.users.RequestDeletion(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to request account deletion"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"deleteAt": at}})
}

func (h *UserDeleteHandler) cancel(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if err := h.users.CancelDeletion(c.Request.Context(), uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to cancel account deletion"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}
