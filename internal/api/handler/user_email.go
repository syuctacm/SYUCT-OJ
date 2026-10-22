package handler

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/mail"
	"github.com/syuctacm/syuct-oj/internal/user"
)

const emailChangeTokenTTL = 24 * time.Hour

type userEmailRepository interface {
	FindByID(context.Context, int64) (*user.User, error)
	FindByMail(context.Context, string) (*user.User, error)
	CreateEmailChangeToken(context.Context, int64, string, time.Duration) (string, error)
	EmailChangeToken(context.Context, string) (int64, string, error)
	DeleteEmailChangeToken(context.Context, string) error
	UpdateEmail(context.Context, int64, string) error
}

type UserEmailHandler struct {
	repo     userEmailRepository
	sessions *auth.SessionManager
	mailer   mail.Sender
}

func NewUserEmailHandler(repo userEmailRepository, sessions *auth.SessionManager, mailer mail.Sender) *UserEmailHandler {
	return &UserEmailHandler{repo: repo, sessions: sessions, mailer: mailer}
}

func (h *UserEmailHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/user/security/email", h.requestChange)
	router.POST("/api/v1/user/security/email/confirm", h.confirmChange)
}

func (h *UserEmailHandler) requestChange(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var request struct {
		Current string `json:"current"`
		Email   string `json:"email"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	account, err := h.repo.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if user.VerifyPassword(account, request.Current) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}
	address := strings.TrimSpace(request.Email)
	if !user.ValidEmail(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的邮箱地址"})
		return
	}
	if strings.EqualFold(address, account.Mail) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新邮箱不能与当前邮箱相同"})
		return
	}
	if existing, err := h.repo.FindByMail(c.Request.Context(), address); err == nil && existing.ID != uid {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被使用"})
		return
	} else if err != nil && !errors.Is(err, user.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to check email"})
		return
	}
	code, err := h.repo.CreateEmailChangeToken(c.Request.Context(), uid, address, emailChangeTokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create email change token"})
		return
	}
	relativeLink := "/home/changeMail/" + url.PathEscape(code)
	if h.mailer == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
			"sent": false, "mail": address, "code": code, "link": relativeLink,
		}})
		return
	}
	absoluteLink := absoluteRequestURL(c) + relativeLink
	body := fmt.Sprintf("<p>您好，%s！</p><p>请点击下面的链接确认修改您的 SYUCT OJ 邮箱：</p><p><a href=\"%s\">%s</a></p><p>如果不是您本人操作，请忽略此邮件。</p>",
		html.EscapeString(account.Uname), html.EscapeString(absoluteLink), html.EscapeString(absoluteLink))
	if err := h.mailer.Send(c.Request.Context(), address, "修改邮箱", body); err != nil {
		_ = h.repo.DeleteEmailChangeToken(c.Request.Context(), code)
		c.JSON(http.StatusBadGateway, gin.H{"error": "unable to send confirmation email"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"sent": true, "mail": address,
	}})
}

func (h *UserEmailHandler) confirmChange(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation code is required"})
		return
	}
	tokenUID, address, err := h.repo.EmailChangeToken(c.Request.Context(), request.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "确认链接无效或已过期"})
		return
	}
	if tokenUID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "confirmation code belongs to another account"})
		return
	}
	if existing, err := h.repo.FindByMail(c.Request.Context(), address); err == nil && existing.ID != uid {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被使用"})
		return
	} else if err != nil && !errors.Is(err, user.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to check email"})
		return
	}
	if err := h.repo.UpdateEmail(c.Request.Context(), uid, address); err != nil {
		if errors.Is(err, user.ErrMailExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被使用"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update email"})
		}
		return
	}
	_ = h.repo.DeleteEmailChangeToken(c.Request.Context(), request.Code)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"email": address}})
}

func (h *UserEmailHandler) requireUser(c *gin.Context) (int64, bool) {
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

func absoluteRequestURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
