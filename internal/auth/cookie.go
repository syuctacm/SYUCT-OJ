package auth

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// cookieSecure controls the Secure attribute of the session cookie. It stays
// off for local HTTP development and is enabled by COOKIE_SECURE=1 in
// production.
var cookieSecure atomic.Bool

func SetCookieSecure(enabled bool) {
	cookieSecure.Store(enabled)
}

func CookieSecure() bool {
	return cookieSecure.Load()
}

// SetSessionCookie writes the session cookie with the flags Hydro's browser
// session expects.
func SetSessionCookie(c *gin.Context, token string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "sid",
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   cookieSecure.Load(),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "sid",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure.Load(),
		SameSite: http.SameSiteLaxMode,
	})
}
