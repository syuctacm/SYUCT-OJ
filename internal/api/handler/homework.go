package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/contest"
)

type HomeworkHandler struct {
	repo *contest.Repository
}

func NewHomeworkHandler(repo *contest.Repository) *HomeworkHandler {
	return &HomeworkHandler{repo: repo}
}

func (h *HomeworkHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/homeworks", h.list)
	router.GET("/api/v1/homeworks/:tid", h.get)
}

func (h *HomeworkHandler) list(c *gin.Context) {
	items, err := h.repo.ListHomework(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list homeworks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *HomeworkHandler) get(c *gin.Context) {
	item, err := h.repo.GetHomework(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "homework not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}
