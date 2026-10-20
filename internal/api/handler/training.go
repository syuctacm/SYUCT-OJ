package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/training"
)

type TrainingHandler struct {
	repo     *training.Repository
	sessions *auth.SessionManager
}

func NewTrainingHandler(repo *training.Repository, sessions *auth.SessionManager) *TrainingHandler {
	return &TrainingHandler{repo: repo, sessions: sessions}
}

func (h *TrainingHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/trainings", h.list)
	router.GET("/api/v1/trainings/:tid", h.get)
	router.POST("/api/v1/trainings/:tid/enroll", h.enroll)
}

func (h *TrainingHandler) list(c *gin.Context) {
	items, err := h.repo.ListFiltered(c.Request.Context(), c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list trainings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *TrainingHandler) get(c *gin.Context) {
	var uid int64
	if h.sessions != nil {
		if session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c)); ok && session.Kind == "user" {
			uid = session.UserID
		}
	}
	item, err := h.repo.Detail(c.Request.Context(), c.Param("tid"), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "training not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *TrainingHandler) enroll(c *gin.Context) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	enrolledNow, err := h.repo.Enroll(c.Request.Context(), c.Param("tid"), session.UserID)
	if err != nil {
		if err == training.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "training not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to enroll training"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"enrolled": true, "changed": enrolledNow}})
}
