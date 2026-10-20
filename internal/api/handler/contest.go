package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/contest"
)

type ContestHandler struct {
	repo *contest.Repository
}

func NewContestHandler(repo *contest.Repository) *ContestHandler {
	return &ContestHandler{repo: repo}
}

func (h *ContestHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/contests", h.list)
	router.GET("/api/v1/contests/:tid", h.get)
}

func (h *ContestHandler) list(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list contests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *ContestHandler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}
