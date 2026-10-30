package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type TrainingHandler struct {
	repo     *training.Repository
	sessions *auth.SessionManager
	users    *user.Repository
}

func NewTrainingHandler(repo *training.Repository, sessions *auth.SessionManager, users *user.Repository) *TrainingHandler {
	return &TrainingHandler{repo: repo, sessions: sessions, users: users}
}

func (h *TrainingHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/trainings", h.list)
	router.POST("/api/v1/trainings", h.create)
	router.GET("/api/v1/trainings/:tid/edit", h.editData)
	router.PATCH("/api/v1/trainings/:tid", h.update)
	router.GET("/api/v1/trainings/:tid", h.get)
	router.POST("/api/v1/trainings/:tid/enroll", h.enroll)
}

func (h *TrainingHandler) create(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var input training.EditInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := h.repo.Create(c.Request.Context(), u.ID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *TrainingHandler) editData(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *TrainingHandler) update(c *gin.Context) {
	if _, ok := h.requireEditor(c); !ok {
		return
	}
	var input training.EditInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	item, err := h.repo.Update(c.Request.Context(), c.Param("tid"), input)
	if err != nil {
		if errors.Is(err, training.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "training not found"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *TrainingHandler) list(c *gin.Context) {
	page, limit := parsePaging(c)
	items, total, err := h.repo.ListFiltered(c.Request.Context(), c.Query("q"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list trainings"})
		return
	}
	progress := map[string]training.ListProgress{}
	if h.sessions != nil {
		if session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c)); ok && session.Kind == "user" && session.UserID > 0 {
			progress, err = h.repo.ListProgress(c.Request.Context(), session.UserID, items)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load training progress"})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": total, "page": page, "limit": limit, "progress": progress},
	})
}

// trainingDetailResponse carries the viewed user's detail plus, when another
// user is being inspected, the caller's own detail for the compare column.
type trainingDetailResponse struct {
	*training.Detail
	ViewedUID int64            `json:"viewedUid"`
	Compare   *training.Detail `json:"compare,omitempty"`
}

func (h *TrainingHandler) get(c *gin.Context) {
	var viewer int64
	if h.sessions != nil {
		if session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c)); ok && session.Kind == "user" {
			viewer = session.UserID
		}
	}
	viewedUID := viewer
	if raw := strings.TrimSpace(c.Query("uid")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		if viewer <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		viewedUID = parsed
	}
	item, err := h.repo.Detail(c.Request.Context(), c.Param("tid"), viewedUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "training not found"})
		return
	}
	response := trainingDetailResponse{Detail: item, ViewedUID: viewedUID}
	if viewer > 0 && viewer != viewedUID {
		compare, err := h.repo.Detail(c.Request.Context(), c.Param("tid"), viewer)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load own progress"})
			return
		}
		response.Compare = compare
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": response})
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

func (h *TrainingHandler) requireAdmin(c *gin.Context) (*user.User, bool) {
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

func (h *TrainingHandler) requireEditor(c *gin.Context) (*training.Training, bool) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "training not found"})
		return nil, false
	}
	if item.Owner != u.ID && u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return item, true
}

func (h *TrainingHandler) currentUser(c *gin.Context) (*user.User, bool) {
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
