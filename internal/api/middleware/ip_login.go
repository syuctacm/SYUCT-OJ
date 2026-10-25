package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/user"
)

const privUserProfile = int64(1 << 2)

// IPAutoLogin implements the onsite-toolkit behavior: a browser API request
// without a valid user session can be bound to the user assigned to its IP.
func IPAutoLogin(enabled bool, users *user.Repository, sessions *auth.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled || users == nil || sessions == nil {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/") || path == "/api/v1/auth/logout" || path == "/api/v1/auth/login" {
			c.Next()
			return
		}
		if session, ok := sessions.GetContext(c.Request.Context(), auth.Token(c)); ok && session.Kind == "user" {
			c.Next()
			return
		}
		uid, found, err := users.IPLoginUID(c.Request.Context(), c.ClientIP())
		if err != nil {
			slog.Warn("unable to resolve IP login", "ip", c.ClientIP(), "error", err)
			c.Next()
			return
		}
		if !found {
			c.Next()
			return
		}
		account, err := users.FindByID(c.Request.Context(), uid)
		if err != nil || account.Priv&privUserProfile == 0 {
			slog.Warn("IP login points to a missing or disabled user", "ip", c.ClientIP(), "uid", uid)
			c.Next()
			return
		}
		token, err := sessions.IssueForUserMeta(c.Request.Context(), uid, c.Request.UserAgent(), c.ClientIP())
		if err != nil {
			slog.Warn("unable to issue IP login session", "uid", uid, "error", err)
			c.Next()
			return
		}
		auth.SetSessionCookie(c, token, sessions.TTL())
		c.Request.AddCookie(&http.Cookie{Name: "sid", Value: token, Path: "/"})
		if err := users.RecordLogin(c.Request.Context(), uid, c.ClientIP()); err != nil {
			slog.Warn("unable to update IP login audit fields", "uid", uid, "error", err)
		}
		c.Next()
	}
}
