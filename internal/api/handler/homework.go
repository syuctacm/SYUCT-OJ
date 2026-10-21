package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type HomeworkHandler struct {
	repo     *contest.Repository
	sessions *auth.SessionManager
	users    *user.Repository
}

func NewHomeworkHandler(repo *contest.Repository, sessions *auth.SessionManager, users *user.Repository) *HomeworkHandler {
	return &HomeworkHandler{repo: repo, sessions: sessions, users: users}
}

func (h *HomeworkHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/homeworks", h.list)
	router.POST("/api/v1/homeworks", h.create)
	router.GET("/api/v1/homeworks/:tid/edit", h.editData)
	router.PATCH("/api/v1/homeworks/:tid", h.update)
	router.GET("/api/v1/homeworks/:tid", h.get)
	router.GET("/api/v1/homeworks/:tid/problems", h.problems)
	router.POST("/api/v1/homeworks/:tid/attend", h.attend)
}

func (h *HomeworkHandler) list(c *gin.Context) {
	page, limit := parsePaging(c)
	items, total, err := h.repo.ListHomeworkFiltered(c.Request.Context(), c.Query("q"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list homeworks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": total, "page": page, "limit": limit},
	})
}

func (h *HomeworkHandler) create(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var input contest.EditInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	input.Rule = "homework"
	item, err := h.repo.Create(c.Request.Context(), u.ID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *HomeworkHandler) editData(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *HomeworkHandler) update(c *gin.Context) {
	if _, ok := h.requireEditor(c); !ok {
		return
	}
	var input contest.EditInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	input.Rule = "homework"
	item, err := h.repo.Update(c.Request.Context(), c.Param("tid"), input)
	if err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "homework not found"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *HomeworkHandler) problems(c *gin.Context) {
	item, problems, err := h.repo.Problems(c.Request.Context(), c.Param("tid"))
	if err != nil || item.Rule != "homework" {
		c.JSON(http.StatusNotFound, gin.H{"error": "homework not found"})
		return
	}
	attended := false
	statuses := map[int64]contest.ProblemStatus{}
	if u, ok := h.currentUser(c); ok {
		attended, err = h.repo.Attended(c.Request.Context(), item.DocID.Hex(), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load homework claim"})
			return
		}
		statuses, err = h.repo.UserProblemStatus(c.Request.Context(), u.ID, item.PIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load problem status"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"homework": item, "problems": problems, "attended": attended, "statuses": statuses},
	})
}

func (h *HomeworkHandler) attend(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	item, err := h.repo.GetHomework(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "homework not found"})
		return
	}
	changed, err := h.repo.Attend(c.Request.Context(), item.DocID.Hex(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to claim homework"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"attended": true, "changed": changed},
	})
}

func (h *HomeworkHandler) get(c *gin.Context) {
	item, err := h.repo.GetHomework(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "homework not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *HomeworkHandler) requireAdmin(c *gin.Context) (*user.User, bool) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	if u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return u, true
}

func (h *HomeworkHandler) requireEditor(c *gin.Context) (*contest.Contest, bool) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	item, err := h.repo.GetHomework(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "homework not found"})
		return nil, false
	}
	if item.Owner != u.ID && u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return item, true
}

func (h *HomeworkHandler) currentUser(c *gin.Context) (*user.User, bool) {
	if h.sessions == nil || h.users == nil {
		return nil, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return nil, false
	}
	u, err := h.users.FindByID(c.Request.Context(), session.UserID)
	if err != nil {
		return nil, false
	}
	return u, true
}
