package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/user"
)

type UserHandler struct {
	users *user.Repository
}

func NewUserHandler(users *user.Repository) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/users/:uid", h.get)
}

func (h *UserHandler) get(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	u, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{
			"id": u.ID, "username": u.Uname, "displayName": u.DisplayName,
			"studentId": u.StudentID, "avatar": u.Avatar, "regAt": u.RegAt,
		},
	})
}
