package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type SubmissionHandler struct {
	repo     *submission.Repository
	sessions *auth.SessionManager
	contests *contest.Repository
	users    *user.Repository
}

func NewSubmissionHandler(repo *submission.Repository, sessions *auth.SessionManager, contests *contest.Repository, users *user.Repository) *SubmissionHandler {
	return &SubmissionHandler{repo: repo, sessions: sessions, contests: contests, users: users}
}

func (h *SubmissionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/submissions", h.create)
	router.GET("/api/v1/submissions", h.list)
	router.GET("/api/v1/submissions/:rid", h.get)
}

func (h *SubmissionHandler) create(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		PID       string `json:"pid"`
		Lang      string `json:"lang"`
		Code      string `json:"code"`
		ContestID string `json:"contestId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.PID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pid, lang and code are required"})
		return
	}
	if request.ContestID != "" && h.contests != nil {
		item, err := h.contests.Get(c.Request.Context(), request.ContestID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "contest not found"})
			return
		}
		now := time.Now()
		if now.Before(item.BeginAt) || now.After(item.EndAt) {
			c.JSON(http.StatusForbidden, gin.H{"error": "contest is not running"})
			return
		}
	}
	record, err := h.repo.Create(c.Request.Context(), uid, request.PID, request.Lang, request.Code, request.ContestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "success", "data": record})
}

func (h *SubmissionHandler) list(c *gin.Context) {
	if _, ok := h.currentUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var filterUID int64
	if value := strings.TrimSpace(c.Query("uidOrName")); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			filterUID = parsed
		} else if h.users != nil {
			found, err := h.users.FindByLogin(c.Request.Context(), value)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": submission.ListResult{Items: []submission.Record{}, Page: page, Limit: limit}})
				return
			}
			filterUID = found.ID
		}
	}
	var status *int
	if value := c.Query("status"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			status = &parsed
		}
	}
	result, err := h.repo.List(c.Request.Context(), submission.ListFilter{
		Page:      page,
		Limit:     limit,
		UID:       filterUID,
		PID:       c.Query("pid"),
		ContestID: c.Query("tid"),
		Lang:      c.Query("lang"),
		Status:    status,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list submissions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *SubmissionHandler) get(c *gin.Context) {
	record, err := h.repo.Get(c.Request.Context(), c.Param("rid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}
	if uid, ok := h.currentUser(c); !ok || uid != record.UID {
		record.Code = ""
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": record})
}

func (h *SubmissionHandler) currentUser(c *gin.Context) (int64, bool) {
	token := auth.Token(c)
	session, ok := h.sessions.Get(token)
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return 0, false
	}
	return session.UserID, true
}
