package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type ContestHandler struct {
	repo     *contest.Repository
	sessions *auth.SessionManager
	users    *user.Repository
}

func NewContestHandler(repo *contest.Repository, sessions *auth.SessionManager, users *user.Repository) *ContestHandler {
	return &ContestHandler{repo: repo, sessions: sessions, users: users}
}

func (h *ContestHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/contests", h.list)
	router.POST("/api/v1/contests", h.create)
	router.GET("/api/v1/contests/:tid/edit", h.editData)
	router.PATCH("/api/v1/contests/:tid", h.update)
	router.GET("/api/v1/contests/:tid", h.get)
	router.POST("/api/v1/contests/:tid/attend", h.attend)
	router.GET("/api/v1/contests/:tid/problems", h.problems)
	router.GET("/api/v1/contests/:tid/scoreboard", h.scoreboard)
	router.GET("/api/v1/contests/:tid/clarifications", h.clarifications)
	router.POST("/api/v1/contests/:tid/clarifications", h.addClarification)
	router.POST("/api/v1/contests/:tid/clarifications/:cid/reply", h.replyClarification)
}

func (h *ContestHandler) list(c *gin.Context) {
	items, err := h.repo.ListFiltered(c.Request.Context(), c.Query("q"), c.Query("rule"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list contests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items, "total": len(items)}})
}

func (h *ContestHandler) create(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var input contest.EditInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if input.Rule == "homework" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "use the homework API"})
		return
	}
	item, err := h.repo.Create(c.Request.Context(), u.ID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *ContestHandler) editData(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *ContestHandler) update(c *gin.Context) {
	if _, ok := h.requireEditor(c); !ok {
		return
	}
	var input contest.EditInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if input.Rule == "homework" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "use the homework API"})
		return
	}
	item, err := h.repo.Update(c.Request.Context(), c.Param("tid"), input)
	if err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *ContestHandler) problems(c *gin.Context) {
	item, problems, err := h.repo.Problems(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	attended := false
	statuses := map[int64]contest.ProblemStatus{}
	if u, ok := h.currentUser(c); ok {
		attended, err = h.repo.Attended(c.Request.Context(), c.Param("tid"), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest attendance"})
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
		"data": gin.H{"contest": item, "problems": problems, "attended": attended, "statuses": statuses},
	})
}

func (h *ContestHandler) scoreboard(c *gin.Context) {
	item, rows, err := h.repo.Scoreboard(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"contest": item, "rows": rows}})
}

type contestDetailResponse struct {
	*contest.Contest
	Attended bool `json:"attended"`
}

func (h *ContestHandler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	attended := false
	if u, ok := h.currentUser(c); ok {
		attended, err = h.repo.Attended(c.Request.Context(), c.Param("tid"), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest attendance"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": contestDetailResponse{Contest: item, Attended: attended},
	})
}

func (h *ContestHandler) attend(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	changed, err := h.repo.Attend(c.Request.Context(), c.Param("tid"), u.ID)
	if err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to attend contest"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"attended": true, "changed": changed},
	})
}

func (h *ContestHandler) clarifications(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	all := u.ID == item.Owner || u.Priv&1 != 0 || containsInt64(item.Maintainer, u.ID)
	items, err := h.repo.ListClarifications(c.Request.Context(), c.Param("tid"), u.ID, all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list clarifications"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": len(items)},
	})
}

func (h *ContestHandler) addClarification(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	if item.Rule == "homework" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "homework does not support clarifications"})
		return
	}
	attended, err := h.repo.Attended(c.Request.Context(), c.Param("tid"), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest attendance"})
		return
	}
	if !attended {
		c.JSON(http.StatusForbidden, gin.H{"error": "contest not attended"})
		return
	}
	now := time.Now()
	if now.Before(item.BeginAt) || now.After(item.EndAt) {
		c.JSON(http.StatusForbidden, gin.H{"error": "contest is not running"})
		return
	}
	var request struct {
		Subject int64  `json:"subject"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	created, err := h.repo.AddClarification(c.Request.Context(), c.Param("tid"), u.ID, request.Subject, request.Content)
	if err != nil {
		if errors.Is(err, contest.ErrInvalidClarificationSubject) || errors.Is(err, contest.ErrEmptyClarificationContent) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to add clarification"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": created})
}

func (h *ContestHandler) replyClarification(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	if u.ID != item.Owner && u.Priv&1 == 0 && !containsInt64(item.Maintainer, u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var request struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	updated, err := h.repo.AddClarificationReply(c.Request.Context(), c.Param("cid"), u.ID, request.Content)
	if err != nil {
		if errors.Is(err, contest.ErrClarificationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "clarification not found"})
			return
		}
		if errors.Is(err, contest.ErrEmptyClarificationContent) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reply clarification"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": updated})
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (h *ContestHandler) requireAdmin(c *gin.Context) (*user.User, bool) {
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

func (h *ContestHandler) requireEditor(c *gin.Context) (*contest.Contest, bool) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return nil, false
	}
	if item.Owner != u.ID && u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return item, true
}

func (h *ContestHandler) currentUser(c *gin.Context) (*user.User, bool) {
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
