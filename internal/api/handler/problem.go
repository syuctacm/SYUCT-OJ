package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

type ProblemHandler struct {
	repo     *problem.Repository
	sessions *auth.SessionManager
}

func NewProblemHandler(repo *problem.Repository, sessions *auth.SessionManager) *ProblemHandler {
	return &ProblemHandler{repo: repo, sessions: sessions}
}

func (h *ProblemHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/problems", h.list)
	router.GET("/api/v1/problem-tags", h.tags)
	router.GET("/api/v1/problem-tags/:tag/ranking", h.tagRanking)
	router.GET("/api/v1/problems/:pid", h.get)
}

func (h *ProblemHandler) list(c *gin.Context) {
	var uid int64
	if h.sessions != nil {
		if session, ok := h.sessions.Get(auth.Token(c)); ok && session.Kind == "user" {
			uid = session.UserID
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var difficulty *int
	if raw := c.Query("difficulty"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			difficulty = &value
		}
	}
	result, err := h.repo.List(c.Request.Context(), problem.ListOptions{
		Page:       page,
		Limit:      limit,
		Query:      c.Query("q"),
		Tag:        c.Query("tag"),
		Difficulty: difficulty,
		Sort:       c.Query("sort"),
		UID:        uid,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list problems"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *ProblemHandler) tags(c *gin.Context) {
	items, err := h.repo.TagStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load tag statistics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items}})
}

func (h *ProblemHandler) tagRanking(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.repo.TagRanking(c.Request.Context(), c.Param("tag"), page, limit, c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load tag ranking"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *ProblemHandler) get(c *gin.Context) {
	pdoc, err := h.repo.Get(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": pdoc})
}
