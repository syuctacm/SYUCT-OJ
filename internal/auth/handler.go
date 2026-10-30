package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/totp"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type UserStore interface {
	FindByLogin(ctx context.Context, identifier string) (*user.User, error)
	FindByID(ctx context.Context, id int64) (*user.User, error)
	FindByMail(ctx context.Context, mail string) (*user.User, error)
	RecordLogin(ctx context.Context, uid int64, ip string) error
	Create(ctx context.Context, input user.RegisterInput) (*user.User, error)
	CreateRegistrationToken(ctx context.Context, mail string, ttl time.Duration) (string, error)
	RegistrationToken(ctx context.Context, code string) (string, error)
	DeleteRegistrationToken(ctx context.Context, code string) error
}

type Handler struct {
	users    UserStore
	sessions *SessionManager
}

func NewHandler(users UserStore, sessions *SessionManager) *Handler {
	return &Handler{users: users, sessions: sessions}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/auth/login", h.login)
	router.GET("/api/v1/auth/me", h.me)
	router.POST("/api/v1/auth/logout", h.logout)
	router.POST("/api/v1/auth/register", h.registerRequest)
	router.GET("/api/v1/auth/register/:code", h.registerInfo)
	router.POST("/api/v1/auth/register/:code", h.registerComplete)
}

type loginRequest struct {
	Username string `form:"uname" json:"username"`
	Password string `form:"password" json:"password"`
	TFA      string `form:"tfa" json:"tfa"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if strings.HasPrefix(c.ContentType(), "application/json") {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
	} else if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, err := h.users.FindByLogin(c.Request.Context(), req.Username)
	if err != nil || user.VerifyPassword(u, req.Password) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if user.IsBanned(u) {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is banned"})
		return
	}
	if u.TFA != "" {
		if strings.TrimSpace(req.TFA) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "2FA required", "tfaRequired": true})
			return
		}
		if !totp.Verify(u.TFA, req.TFA, time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid 2FA code", "tfaRequired": true})
			return
		}
	}
	token, err := h.sessions.IssueForUserMeta(c.Request.Context(), u.ID, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}
	SetSessionCookie(c, token, h.sessions.TTL())
	if err := h.users.RecordLogin(c.Request.Context(), u.ID, c.ClientIP()); err != nil {
		slog.Warn("unable to update login audit fields", "uid", u.ID, "error", err)
	}
	c.JSON(http.StatusOK, gin.H{"user": PublicUser(u)})
}

func (h *Handler) me(c *gin.Context) {
	token := Token(c)
	session, ok := h.sessions.GetContext(c.Request.Context(), token)
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	u, err := h.users.FindByID(c.Request.Context(), session.UserID)
	if err != nil || user.IsBanned(u) {
		h.sessions.RevokeContext(c.Request.Context(), token)
		ClearSessionCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": PublicUser(u)})
}

func (h *Handler) logout(c *gin.Context) {
	token := Token(c)
	if token != "" {
		h.sessions.RevokeContext(c.Request.Context(), token)
	}
	ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func Token(c *gin.Context) string {
	if cookie, err := c.Cookie("sid"); err == nil {
		return cookie
	}
	authorization := c.GetHeader("Authorization")
	if strings.HasPrefix(authorization, "Bearer ") {
		return strings.TrimPrefix(authorization, "Bearer ")
	}
	return ""
}

// PublicUser is the safe user payload shared by password and WebAuthn login.
func PublicUser(u *user.User) gin.H {
	return gin.H{
		"id":              u.ID,
		"username":        u.Uname,
		"email":           u.Mail,
		"displayName":     u.DisplayName,
		"studentId":       u.StudentID,
		"avatar":          u.Avatar,
		"priv":            u.Priv,
		"bio":             u.Bio,
		"qq":              u.QQ,
		"gender":          u.Gender,
		"school":          u.School,
		"backgroundImage": u.BackgroundImage,
		"viewLang":        u.ViewLang,
		"fontFamily":      u.FontFamily,
		"codeFontFamily":  u.CodeFontFamily,
		"codeLang":        u.CodeLang,
		"codeTemplate":    u.CodeTemplate,
		"skipAnimate":     u.SkipAnimate,
		"tfa":             u.TFA != "",
	}
}
