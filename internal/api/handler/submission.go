package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/submission"
)

type SubmissionHandler struct {
	repo     *submission.Repository
	sessions *auth.SessionManager
}

func NewSubmissionHandler(repo *submission.Repository, sessions *auth.SessionManager) *SubmissionHandler {
	return &SubmissionHandler{repo: repo, sessions: sessions}
}

func (h *SubmissionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/submissions", h.create)
	router.GET("/api/v1/submissions/:rid", h.get)
}

func (h *SubmissionHandler) create(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		PID  string `json:"pid"`
		Lang string `json:"lang"`
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.PID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pid, lang and code are required"})
		return
	}
	record, err := h.repo.Create(c.Request.Context(), uid, request.PID, request.Lang, request.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "success", "data": record})
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
