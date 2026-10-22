package handler

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/totp"
	"github.com/syuctacm/syuct-oj/internal/user"
)

func (h *UserHandler) tfaSetup(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	account, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if account.TFA != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "2FA is already enabled"})
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to generate 2FA secret"})
		return
	}
	uri := totp.ProvisioningURI("SYUCT OJ", account.Uname, secret)
	qr, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to generate 2FA QR code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"secret": secret,
		"uri":    uri,
		"qr":     "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr),
	}})
}

func (h *UserHandler) tfaEnable(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.Secret = strings.ToUpper(strings.TrimSpace(request.Secret))
	if err := totp.ValidateSecret(request.Secret); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 2FA secret"})
		return
	}
	account, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if account.TFA != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "2FA is already enabled"})
		return
	}
	if !totp.Verify(request.Secret, request.Code, time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 2FA code"})
		return
	}
	if err := h.users.UpdateSettings(c.Request.Context(), uid, bson.M{"tfa": request.Secret}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to enable 2FA"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"tfa": true}})
}

func (h *UserHandler) tfaDisable(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		Current string `json:"current"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	account, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if user.VerifyPassword(account, request.Current) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}
	if account.TFA == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "2FA is not enabled"})
		return
	}
	if err := h.users.UpdateSettings(c.Request.Context(), uid, bson.M{"tfa": ""}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to disable 2FA"})
		return
	}
	c.Status(http.StatusNoContent)
}
