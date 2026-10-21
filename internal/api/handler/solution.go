package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

// SolutionHandler serves the per-problem solution (题解) area. The page markup
// comes from the pinned ui-default template; this handler only supplies data.
type SolutionHandler struct {
	repo     *problem.Repository
	sessions *auth.SessionManager
	users    *user.Repository
}

func NewSolutionHandler(repo *problem.Repository, sessions *auth.SessionManager, users *user.Repository) *SolutionHandler {
	return &SolutionHandler{repo: repo, sessions: sessions, users: users}
}

func (h *SolutionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/problems/:pid/solutions", h.list)
	router.POST("/api/v1/problems/:pid/solutions", h.create)
	router.PATCH("/api/v1/problems/:pid/solutions/:sid", h.update)
	router.DELETE("/api/v1/problems/:pid/solutions/:sid", h.delete)
	router.POST("/api/v1/problems/:pid/solutions/:sid/vote", h.vote)
	router.POST("/api/v1/problems/:pid/solutions/:sid/replies", h.reply)
	router.PATCH("/api/v1/problems/:pid/solutions/:sid/replies/:rid", h.updateReply)
	router.DELETE("/api/v1/problems/:pid/solutions/:sid/replies/:rid", h.deleteReply)
}

// list serves the solution list, or a single solution when `sid` is given,
// mirroring the upstream `problem_solution_detail` route.
func (h *SolutionHandler) list(c *gin.Context) {
	pdoc, ok := h.loadProblem(c)
	if !ok {
		return
	}
	uid, _ := h.currentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, total, err := h.repo.ListSolutions(c.Request.Context(), pdoc.DocID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load solutions"})
		return
	}
	if sidParam := strings.TrimSpace(c.Query("sid")); sidParam != "" {
		sid, err := bson.ObjectIDFromHex(sidParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid solution id"})
			return
		}
		item, err := h.repo.Solution(c.Request.Context(), sid)
		if err != nil || item.PID != pdoc.DocID {
			c.JSON(http.StatusNotFound, gin.H{"error": "solution not found"})
			return
		}
		items = []problem.Solution{*item}
	}
	sids := make([]bson.ObjectID, 0, len(items))
	for _, item := range items {
		sids = append(sids, item.ID)
	}
	votes, err := h.repo.SolutionVotes(c.Request.Context(), uid, sids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load votes"})
		return
	}
	ids := make([]int64, 0, len(items)+1)
	seen := map[int64]bool{}
	addUser := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	// The sidebar renders the problem owner through the same user dictionary.
	addUser(pdoc.Owner)
	for _, item := range items {
		addUser(item.Owner)
		for _, reply := range item.Replies {
			addUser(reply.Owner)
		}
	}
	users := []gin.H{}
	if len(ids) > 0 && h.users != nil {
		accounts, err := h.users.FindByIDs(c.Request.Context(), ids)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load solutions"})
			return
		}
		for _, account := range accounts {
			users = append(users, gin.H{
				"id": account.ID, "username": account.Uname,
				"displayName": account.DisplayName, "avatar": account.Avatar,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": items, "total": total, "page": page, "votes": votes, "users": users,
		"problem": gin.H{"docId": pdoc.DocID, "pid": pdoc.PID, "title": pdoc.Title, "owner": pdoc.Owner},
	}})
}

func (h *SolutionHandler) create(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	pdoc, ok := h.loadProblem(c)
	if !ok {
		return
	}
	content, ok := h.content(c)
	if !ok {
		return
	}
	item, err := h.repo.CreateSolution(c.Request.Context(), pdoc.DocID, uid, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create solution"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *SolutionHandler) update(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, ok := h.loadOwnedSolution(c, uid)
	if !ok {
		return
	}
	content, ok := h.content(c)
	if !ok {
		return
	}
	if err := h.repo.UpdateSolution(c.Request.Context(), item.ID, content); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "solution not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SolutionHandler) delete(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	item, ok := h.loadOwnedSolution(c, uid)
	if !ok {
		return
	}
	if err := h.repo.DeleteSolution(c.Request.Context(), item.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "solution not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SolutionHandler) vote(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	sid, ok := h.solutionID(c)
	if !ok {
		return
	}
	var request struct {
		Value int64 `json:"value"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || (request.Value != 0 && request.Value != 1 && request.Value != -1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vote"})
		return
	}
	total, err := h.repo.VoteSolution(c.Request.Context(), sid, uid, request.Value)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "solution not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"vote": total, "value": request.Value}})
}

func (h *SolutionHandler) reply(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	sid, ok := h.solutionID(c)
	if !ok {
		return
	}
	content, ok := h.content(c)
	if !ok {
		return
	}
	item, err := h.repo.AddSolutionReply(c.Request.Context(), sid, uid, content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "solution not found"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": item})
}

func (h *SolutionHandler) updateReply(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	sid, rid, ok := h.solutionReplyIDs(c)
	if !ok {
		return
	}
	content, ok := h.content(c)
	if !ok {
		return
	}
	_, reply, err := h.repo.SolutionReply(c.Request.Context(), sid, rid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if reply.Owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.repo.UpdateSolutionReply(c.Request.Context(), sid, rid, content); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SolutionHandler) deleteReply(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	sid, rid, ok := h.solutionReplyIDs(c)
	if !ok {
		return
	}
	_, reply, err := h.repo.SolutionReply(c.Request.Context(), sid, rid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	if reply.Owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.repo.DeleteSolutionReply(c.Request.Context(), sid, rid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reply not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SolutionHandler) loadProblem(c *gin.Context) (*problem.Problem, bool) {
	pdoc, err := h.repo.Get(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return nil, false
	}
	return pdoc, true
}

// loadOwnedSolution loads the solution and checks the caller may edit it:
// authors and administrators only, mirroring Hydro's self permissions.
func (h *SolutionHandler) loadOwnedSolution(c *gin.Context, uid int64) (*problem.Solution, bool) {
	sid, ok := h.solutionID(c)
	if !ok {
		return nil, false
	}
	item, err := h.repo.Solution(c.Request.Context(), sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "solution not found"})
		return nil, false
	}
	if item.Owner != uid && !h.isAdmin(c.Request.Context(), uid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return item, true
}

func (h *SolutionHandler) solutionID(c *gin.Context) (bson.ObjectID, bool) {
	sid, err := bson.ObjectIDFromHex(c.Param("sid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid solution id"})
		return bson.NilObjectID, false
	}
	return sid, true
}

func (h *SolutionHandler) solutionReplyIDs(c *gin.Context) (bson.ObjectID, bson.ObjectID, bool) {
	sid, ok := h.solutionID(c)
	if !ok {
		return bson.NilObjectID, bson.NilObjectID, false
	}
	rid, err := bson.ObjectIDFromHex(c.Param("rid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reply id"})
		return bson.NilObjectID, bson.NilObjectID, false
	}
	return sid, rid, true
}

func (h *SolutionHandler) content(c *gin.Context) (string, bool) {
	var request struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return "", false
	}
	return request.Content, true
}

func (h *SolutionHandler) isAdmin(ctx context.Context, uid int64) bool {
	if h.users == nil || uid <= 0 {
		return false
	}
	account, err := h.users.FindByID(ctx, uid)
	return err == nil && account.Priv&1 != 0
}

func (h *SolutionHandler) requireUser(c *gin.Context) (int64, bool) {
	uid, ok := h.currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	return uid, true
}

func (h *SolutionHandler) currentUserID(c *gin.Context) (int64, bool) {
	if h.sessions == nil {
		return 0, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return 0, false
	}
	return session.UserID, true
}
