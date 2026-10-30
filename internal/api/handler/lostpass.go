package handler

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/mail"
	"github.com/syuctacm/syuct-oj/internal/user"
)

const lostPassTokenTTL = 24 * time.Hour

type lostPassRepository interface {
	FindByMail(context.Context, string) (*user.User, error)
	FindByID(context.Context, int64) (*user.User, error)
	CreateLostPassToken(context.Context, int64, time.Duration) (string, error)
	LostPassToken(context.Context, string) (int64, error)
	DeleteLostPassToken(context.Context, string) error
	ResetPassword(context.Context, int64, string) error
}

type LostPassHandler struct {
	repo     lostPassRepository
	sessions *auth.SessionManager
	mailer   mail.Sender
}

func NewLostPassHandler(repo lostPassRepository, sessions *auth.SessionManager, mailer mail.Sender) *LostPassHandler {
	return &LostPassHandler{repo: repo, sessions: sessions, mailer: mailer}
}

func (h *LostPassHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/auth/lostpass", h.request)
	router.GET("/api/v1/auth/lostpass/:code", h.info)
	router.POST("/api/v1/auth/lostpass/:code", h.reset)
}

func (h *LostPassHandler) request(c *gin.Context) {
	var request struct {
		Mail string `json:"mail"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	address := strings.TrimSpace(request.Mail)
	if !user.ValidEmail(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的邮箱地址"})
		return
	}
	account, err := h.repo.FindByMail(c.Request.Context(), address)
	if errors.Is(err, user.ErrNotFound) {
		// Keep the public response identical so this endpoint cannot be used to
		// enumerate registered email addresses.
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"sent": true}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to check email"})
		return
	}
	code, err := h.repo.CreateLostPassToken(c.Request.Context(), account.ID, lostPassTokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create reset token"})
		return
	}
	relativeLink := "/lostpass/" + url.PathEscape(code)
	if h.mailer == nil {
		// This mirrors the registration/email-change development behavior. A
		// configured deployment never returns the reset token to the browser.
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
			"sent": false, "link": relativeLink, "code": code,
		}})
		return
	}
	absoluteLink := absoluteRequestURL(c) + relativeLink
	body := fmt.Sprintf("<p>您好，%s！</p><p>请点击下面的链接重置您的 SYUCT OJ 密码：</p><p><a href=\"%s\">%s</a></p><p>如果不是您本人操作，请忽略此邮件。</p>",
		html.EscapeString(account.Uname), html.EscapeString(absoluteLink), html.EscapeString(absoluteLink))
	if err := h.mailer.Send(c.Request.Context(), address, "重置密码", body); err != nil {
		_ = h.repo.DeleteLostPassToken(c.Request.Context(), code)
		c.JSON(http.StatusBadGateway, gin.H{"error": "unable to send reset email"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"sent": true}})
}

func (h *LostPassHandler) info(c *gin.Context) {
	uid, err := h.repo.LostPassToken(c.Request.Context(), c.Param("code"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "重置链接无效或已过期"})
		return
	}
	account, err := h.repo.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"username": account.Uname}})
}

func (h *LostPassHandler) reset(c *gin.Context) {
	var request struct {
		Password       string `json:"password"`
		VerifyPassword string `json:"verifyPassword"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if !user.ValidPassword(request.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度需为 6-255 个字符"})
		return
	}
	if request.Password != request.VerifyPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的密码不一致"})
		return
	}
	uid, err := h.repo.LostPassToken(c.Request.Context(), c.Param("code"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "重置链接无效或已过期"})
		return
	}
	// Consume the token before changing the password so a retry cannot reuse it.
	if err := h.repo.DeleteLostPassToken(c.Request.Context(), c.Param("code")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to consume reset token"})
		return
	}
	if err := h.repo.ResetPassword(c.Request.Context(), uid, request.Password); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reset password"})
		return
	}
	if h.sessions != nil {
		if err := h.sessions.RevokeUser(c.Request.Context(), uid); err != nil {
			slog.Warn("unable to revoke sessions after password reset", "uid", uid, "error", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}
