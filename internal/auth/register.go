package auth

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/user"
)

// registrationTokenTTL matches the upstream unsaved-session window.
const registrationTokenTTL = 24 * time.Hour

// registerRequest issues the registration token for an email address. Without
// SMTP the token is returned to the browser so it can continue at
// /register/<code>, which is the upstream behaviour when mail is not configured.
func (h *Handler) registerRequest(c *gin.Context) {
	var request struct {
		Mail string `json:"mail"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	address := strings.TrimSpace(request.Mail)
	if !validEmail(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的邮箱地址"})
		return
	}
	if _, err := h.users.FindByMail(c.Request.Context(), address); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已注册"})
		return
	} else if !errors.Is(err, user.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to check email"})
		return
	}
	code, err := h.users.CreateRegistrationToken(c.Request.Context(), address, registrationTokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create registration token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"mail": address, "code": code}})
}

func (h *Handler) registerInfo(c *gin.Context) {
	address, err := h.users.RegistrationToken(c.Request.Context(), c.Param("code"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "注册链接无效或已过期"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"mail": address}})
}

// registerComplete creates the account and logs it in, mirroring
// UserRegisterWithCodeHandler.post.
func (h *Handler) registerComplete(c *gin.Context) {
	var request struct {
		Username       string `json:"uname"`
		Password       string `json:"password"`
		VerifyPassword string `json:"verifyPassword"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	code := c.Param("code")
	address, err := h.users.RegistrationToken(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "注册链接无效或已过期"})
		return
	}
	username := strings.TrimSpace(request.Username)
	if !validUsername(username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名需为 3-31 个字符（或 2 个汉字）"})
		return
	}
	if !validPassword(request.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度需为 6-255 个字符"})
		return
	}
	if request.Password != request.VerifyPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次输入的密码不一致"})
		return
	}
	account, err := h.users.Create(c.Request.Context(), user.RegisterInput{
		Mail:     address,
		Username: username,
		Password: request.Password,
		IP:       c.ClientIP(),
	})
	if err != nil {
		switch {
		case errors.Is(err, user.ErrMailExists):
			c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已注册"})
		case errors.Is(err, user.ErrUsernameExists):
			c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被占用"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create account"})
		}
		return
	}
	_ = h.users.DeleteRegistrationToken(c.Request.Context(), code)
	token, err := h.sessions.IssueForUserMeta(c.Request.Context(), account.ID, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}
	SetSessionCookie(c, token, h.sessions.TTL())
	c.JSON(http.StatusCreated, gin.H{"user": publicUser(account)})
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}
	return strings.EqualFold(address.Address, value) && strings.Contains(value, "@")
}

// validUsername mirrors Hydro's Types.Username: 3..31 characters, or exactly
// two Chinese characters.
func validUsername(value string) bool {
	runes := []rune(value)
	if len(runes) >= 3 && len(runes) <= 31 {
		return true
	}
	if len(runes) != 2 {
		return false
	}
	for _, char := range runes {
		if !unicode.Is(unicode.Han, char) {
			return false
		}
	}
	return true
}

func validPassword(value string) bool {
	size := len([]byte(value))
	return size >= 6 && size <= 255
}
