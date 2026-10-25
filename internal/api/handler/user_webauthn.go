package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/user"
	"github.com/syuctacm/syuct-oj/internal/webauthn"
)

type WebAuthnHandler struct {
	service  *webauthn.Service
	users    *user.Repository
	sessions *auth.SessionManager
}

func NewWebAuthnHandler(service *webauthn.Service, users *user.Repository, sessions *auth.SessionManager) *WebAuthnHandler {
	return &WebAuthnHandler{service: service, users: users, sessions: sessions}
}

func (h *WebAuthnHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/auth/webauthn/login/begin", h.beginLogin)
	router.POST("/api/v1/auth/webauthn/login/finish", h.finishLogin)
	router.POST("/api/v1/user/security/webauthn/register/begin", h.beginRegistration)
	router.POST("/api/v1/user/security/webauthn/register/finish", h.finishRegistration)
	router.GET("/api/v1/user/security/webauthn", h.listCredentials)
	router.DELETE("/api/v1/user/security/webauthn/:id", h.deleteCredential)
}

type webAuthnFinishRequest struct {
	Token      string          `json:"token"`
	Name       string          `json:"name"`
	Credential json.RawMessage `json:"credential"`
}

func (h *WebAuthnHandler) beginLogin(c *gin.Context) {
	var request struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Username) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}
	options, token, err := h.service.BeginLogin(c.Request.Context(), strings.TrimSpace(request.Username))
	if err != nil {
		if errors.Is(err, user.ErrNotFound) || errors.Is(err, webauthn.ErrNoCredentials) {
			c.JSON(http.StatusNotFound, gin.H{"error": "passkey login is not available for this account"})
			return
		}
		slog.Error("unable to begin WebAuthn login", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to start passkey login"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"options": options, "token": token})
}

func (h *WebAuthnHandler) finishLogin(c *gin.Context) {
	var request webAuthnFinishRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Token) == "" || len(request.Credential) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey response"})
		return
	}
	bound := webauthn.BindRequest(c.Request, request.Credential)
	account, err := h.service.FinishLogin(c.Request.Context(), strings.TrimSpace(request.Token), bound)
	if err != nil {
		slog.Warn("WebAuthn login failed", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "passkey verification failed"})
		return
	}
	token, err := h.sessions.IssueForUserMeta(c.Request.Context(), account.ID, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		slog.Error("unable to issue WebAuthn session", "uid", account.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}
	auth.SetSessionCookie(c, token, h.sessions.TTL())
	if err := h.users.RecordLogin(c.Request.Context(), account.ID, c.ClientIP()); err != nil {
		slog.Warn("unable to update login audit fields", "uid", account.ID, "error", err)
	}
	c.JSON(http.StatusOK, gin.H{"user": auth.PublicUser(account)})
}

func (h *WebAuthnHandler) beginRegistration(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	options, token, err := h.service.BeginRegistration(c.Request.Context(), uid)
	if err != nil {
		slog.Error("unable to begin WebAuthn registration", "uid", uid, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to start passkey registration"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"options": options, "token": token})
}

func (h *WebAuthnHandler) finishRegistration(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var request webAuthnFinishRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Token) == "" || len(request.Credential) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey response"})
		return
	}
	name := strings.TrimSpace(request.Name)
	if len([]rune(name)) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "passkey name is too long"})
		return
	}
	bound := webauthn.BindRequest(c.Request, request.Credential)
	info, err := h.service.FinishRegistration(c.Request.Context(), uid, strings.TrimSpace(request.Token), name, bound)
	if err != nil {
		switch {
		case errors.Is(err, webauthn.ErrChallengeInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "passkey challenge is invalid or expired"})
		case errors.Is(err, webauthn.ErrCredentialExists):
			c.JSON(http.StatusConflict, gin.H{"error": "passkey is already registered"})
		default:
			slog.Warn("WebAuthn registration failed", "uid", uid, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "passkey registration failed"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": info})
}

func (h *WebAuthnHandler) listCredentials(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	items, err := h.service.ListCredentials(c.Request.Context(), uid)
	if err != nil {
		slog.Error("unable to list WebAuthn credentials", "uid", uid, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load passkeys"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items}})
}

func (h *WebAuthnHandler) deleteCredential(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if err := h.service.DeleteCredential(c.Request.Context(), uid, c.Param("id")); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "passkey not found"})
			return
		}
		slog.Error("unable to delete WebAuthn credential", "uid", uid, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to delete passkey"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WebAuthnHandler) requireUser(c *gin.Context) (int64, bool) {
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
