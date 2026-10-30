package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

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
	router.GET("/api/v1/homeworks/:tid/code", h.exportCode)
}

func (h *HomeworkHandler) list(c *gin.Context) {
	page, limit := parsePaging(c)
	uid := int64(0)
	groups := []string{}
	privileged := false
	if u, ok := h.currentUser(c); ok {
		uid = u.ID
		privileged = u.Priv&1 != 0
		if h.users != nil {
			var groupErr error
			groups, groupErr = h.users.GroupNamesForUID(c.Request.Context(), u.ID)
			if groupErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load user groups"})
				return
			}
		}
	}
	access := contest.AssignFilter(uid, groups, privileged)
	items, total, err := h.repo.ListHomeworkVisibleFiltered(
		c.Request.Context(), c.Query("q"), c.Query("group"), access, page, limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list homeworks"})
		return
	}
	if uid > 0 {
		docIDs := make([]bson.ObjectID, 0, len(items))
		for _, item := range items {
			docIDs = append(docIDs, item.DocID)
		}
		statuses, statusErr := h.repo.UserStatuses(c.Request.Context(), uid, docIDs)
		if statusErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load homework claims"})
			return
		}
		for i := range items {
			items[i].Attended = statuses[items[i].DocID.Hex()].Attend == 1
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": total, "page": page, "limit": limit, "groups": groups},
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
	var attendeeStatus *contest.AttendeeStatus
	if u, ok := h.currentUser(c); ok {
		attendeeStatus, err = h.repo.StartIfAttended(c.Request.Context(), item.DocID.Hex(), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load homework claim"})
			return
		}
		attended = attendeeStatus != nil && attendeeStatus.Attend == 1
		statuses, err = h.repo.UserProblemStatus(c.Request.Context(), u.ID, item.PIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load problem status"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{
			"homework": item, "problems": problems, "attended": attended,
			"status": attendeeStatus, "statuses": statuses,
		},
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

// exportCode mirrors the upstream homework_code handler.
func (h *HomeworkHandler) exportCode(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	u, ok := h.currentUser(c)
	if !ok {
		return
	}
	exportCodeArchive(c, h.repo, item, u.ID, u.Priv&1 != 0)
}
