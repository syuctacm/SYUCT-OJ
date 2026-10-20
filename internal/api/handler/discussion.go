package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/discussion"
)

type DiscussionHandler struct {
	repo *discussion.Repository
}

func NewDiscussionHandler(repo *discussion.Repository) *DiscussionHandler {
	return &DiscussionHandler{repo: repo}
}

func (h *DiscussionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/discussions", h.list)
	router.GET("/api/v1/discussions/:did", h.get)
}

func (h *DiscussionHandler) list(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list discussions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *DiscussionHandler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("did"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "discussion not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}
