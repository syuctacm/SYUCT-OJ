package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/submission"
)

type SubmissionHandler struct {
	repo     *submission.Repository
	sessions *auth.SessionManager
	contests *contest.Repository
}

func NewSubmissionHandler(repo *submission.Repository, sessions *auth.SessionManager, contests *contest.Repository) *SubmissionHandler {
	return &SubmissionHandler{repo: repo, sessions: sessions, contests: contests}
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
		if _, err := h.contests.Get(c.Request.Context(), request.ContestID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "contest not found"})
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
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.repo.ListByUser(c.Request.Context(), uid, page, limit)
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
