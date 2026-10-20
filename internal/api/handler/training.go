package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/training"
)

type TrainingHandler struct {
	repo *training.Repository
}

func NewTrainingHandler(repo *training.Repository) *TrainingHandler {
	return &TrainingHandler{repo: repo}
}

func (h *TrainingHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/trainings", h.list)
	router.GET("/api/v1/trainings/:tid", h.get)
}

func (h *TrainingHandler) list(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list trainings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *TrainingHandler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "training not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}
