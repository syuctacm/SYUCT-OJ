package handler

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/user"
)

// securitySessions lists the caller's browser sessions.
func (h *UserHandler) securitySessions(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	sessions, err := h.sessions.ListByUser(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load sessions"})
		return
	}
	current := h.sessions.Digest(auth.Token(c))
	items := make([]gin.H, 0, len(sessions))
	for _, session := range sessions {
		// Sessions issued before metadata existed have no updateAt; fall back to
		// their creation time instead of showing the zero time.
		updatedAt := session.UpdateAt
		if updatedAt.IsZero() {
			updatedAt = session.CreatedAt
		}
		items = append(items, gin.H{
			"id": session.ID, "createdAt": session.CreatedAt, "updateAt": updatedAt,
			"expiresAt": session.ExpiresAt, "ip": session.IP, "ua": session.UA,
			"current": session.ID == current,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items}})
}

// securityRevokeSession logs out one of the caller's sessions.
func (h *UserHandler) securityRevokeSession(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.sessions.RevokeDigest(c.Request.Context(), uid, c.Param("digest")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// securityRevokeAll logs out every session of the caller.
func (h *UserHandler) securityRevokeAll(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.sessions.RevokeUser(c.Request.Context(), uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to revoke sessions"})
		return
	}
	auth.ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// securityChangePassword mirrors HomeSecurityHandler.postChangePassword: check
// the current password, store the new one and drop every session.
func (h *UserHandler) securityChangePassword(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		Current        string `json:"current"`
		Password       string `json:"password"`
		VerifyPassword string `json:"verifyPassword"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if request.Password == "" || request.Password != request.VerifyPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的新密码不一致"})
		return
	}
	account, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := user.VerifyPassword(account, request.Current); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "当前密码不正确"})
		return
	}
	salt, ok := newSalt()
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to change password"})
		return
	}
	if err := h.users.UpdatePassword(c.Request.Context(), uid, salt, user.HashPassword(request.Password, salt)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to change password"})
		return
	}
	_ = h.sessions.RevokeUser(c.Request.Context(), uid)
	auth.ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func newSalt() (string, bool) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", false
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buffer), "="), true
}
