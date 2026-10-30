package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/user"
)

// ContestAssign enforces the `assign` group list on every contest/homework
// subresource. List endpoints are filtered in their handlers because they do
// not carry a :tid path segment.
func ContestAssign(contests *contest.Repository, users *user.Repository, sessions *auth.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if contests == nil {
			c.Next()
			return
		}
		parts := strings.Split(strings.Trim(c.Request.URL.Path, "/"), "/")
		if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" {
			c.Next()
			return
		}
		resource, tid := parts[2], parts[3]
		if resource != "contests" && resource != "homeworks" {
			c.Next()
			return
		}
		item, err := contests.Get(c.Request.Context(), tid)
		if err != nil {
			message := "contest not found"
			if resource == "homeworks" {
				message = "homework not found"
			}
			c.JSON(http.StatusNotFound, gin.H{"error": message})
			c.Abort()
			return
		}
		if resource == "contests" && item.Rule == "homework" {
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
			c.Abort()
			return
		}
		if resource == "homeworks" && item.Rule != "homework" {
			c.JSON(http.StatusNotFound, gin.H{"error": "homework not found"})
			c.Abort()
			return
		}
		if len(item.Assign) == 0 {
			c.Next()
			return
		}

		uid := int64(0)
		privileged := false
		if sessions != nil {
			if session, ok := sessions.GetContext(c.Request.Context(), auth.Token(c)); ok && session.Kind == "user" && session.UserID > 0 {
				uid = session.UserID
				if users != nil {
					if account, err := users.FindByID(c.Request.Context(), uid); err == nil {
						privileged = account.Priv&1 != 0
					}
				}
			}
		}
		if contest.HasAssignAccess(item, uid, nil, privileged) {
			c.Next()
			return
		}
		groups := []string{}
		if uid > 0 && users != nil {
			var groupErr error
			groups, groupErr = users.GroupNamesForUID(c.Request.Context(), uid)
			if groupErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load user groups"})
				c.Abort()
				return
			}
		}
		if !contest.HasAssignAccess(item, uid, groups, privileged) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not assigned to this contest"})
			c.Abort()
			return
		}
		c.Next()
	}
}
