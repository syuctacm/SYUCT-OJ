package handler

import (
	"net/http"
	"strconv"
	"strings"

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
	router.GET("/api/v1/users", h.list)
	router.GET("/api/v1/groups", h.groups)
	router.GET("/api/v1/users/:uid", h.get)
	router.GET("/api/v1/users/:uid/activity", h.activity)
}

type userChoice struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

type groupChoice struct {
	Name string  `json:"name"`
	UIDs []int64 `json:"uids"`
}

func (h *UserHandler) list(c *gin.Context) {
	rawIDs := c.Query("auto")
	if rawIDs == "" {
		rawIDs = c.Query("ids")
	}
	var items []user.User
	var err error
	if rawIDs != "" {
		ids := make([]int64, 0)
		for _, value := range strings.Split(rawIDs, ",") {
			id, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if parseErr == nil && id > 0 {
				ids = append(ids, id)
			}
		}
		items, err = h.users.FindByIDs(c.Request.Context(), ids)
	} else {
		items, err = h.users.Search(c.Request.Context(), c.Query("search"), 20)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to search users"})
		return
	}
	choices := make([]userChoice, 0, len(items))
	for _, item := range items {
		choices = append(choices, userChoice{
			ID: item.ID, Username: item.Uname, DisplayName: item.DisplayName, Avatar: item.Avatar,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": choices}})
}

func (h *UserHandler) groups(c *gin.Context) {
	rawNames := c.Query("names")
	var items []user.Group
	var err error
	if rawNames != "" {
		items, err = h.users.FindGroupsByNames(c.Request.Context(), strings.Split(rawNames, ","))
	} else {
		items, err = h.users.SearchGroups(c.Request.Context(), c.Query("search"), 20)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to search groups"})
		return
	}
	choices := make([]groupChoice, 0, len(items))
	for _, item := range items {
		choices = append(choices, groupChoice{Name: item.Name, UIDs: item.UIDs})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": choices}})
}

func (h *UserHandler) activity(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	activity, err := h.users.Activity(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load activity"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": activity})
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
	submit, accepted, err := h.users.Statistics(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load user statistics"})
		return
	}
	acceptedProblems, err := h.users.AcceptedProblems(c.Request.Context(), uid, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load accepted problems"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{
			"id": u.ID, "username": u.Uname, "displayName": u.DisplayName,
			"studentId": u.StudentID, "avatar": u.Avatar, "regAt": u.RegAt,
			"loginAt": u.LoginAt, "bio": u.Bio, "nSubmit": submit, "nAccept": accepted,
			"acceptedProblems": acceptedProblems,
		},
	})
}
