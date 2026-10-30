package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/filemeta"
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
	router.POST("/api/v1/contests/:tid/early-end", h.earlyEnd)
	router.GET("/api/v1/contests/:tid/problems", h.problems)
	router.GET("/api/v1/contests/:tid/scoreboard", h.scoreboard)
	router.POST("/api/v1/contests/:tid/unlock", h.unlock)
	router.GET("/api/v1/contests/:tid/attendees", h.attendees)
	router.POST("/api/v1/contests/:tid/attendees", h.addAttendees)
	router.PATCH("/api/v1/contests/:tid/attendees/:uid", h.setAttendeeRank)
	router.DELETE("/api/v1/contests/:tid/attendees/:uid", h.cancelAttendee)
	router.POST("/api/v1/contests/:tid/attendees/:uid/resume", h.resumeAttendee)
	router.GET("/api/v1/contests/:tid/code", h.exportCode)
	router.POST("/api/v1/contests/:tid/score", h.setScore)
	router.GET("/api/v1/contests/:tid/balloons", h.balloons)
	router.PUT("/api/v1/contests/:tid/balloons/colors", h.setBalloonColors)
	router.POST("/api/v1/contests/:tid/balloons/:bid/sent", h.markBalloonSent)
	router.GET("/api/v1/contests/:tid/print/tasks", h.printTasks)
	router.POST("/api/v1/contests/:tid/print/tasks", h.addPrintTask)
	router.POST("/api/v1/contests/:tid/print/allocate", h.allocatePrintTask)
	router.POST("/api/v1/contests/:tid/print/tasks/:pid/status", h.updatePrintTask)
	router.GET("/api/v1/contests/:tid/clarifications", h.clarifications)
	router.POST("/api/v1/contests/:tid/clarifications", h.addClarification)
	router.POST("/api/v1/contests/:tid/clarifications/broadcast", h.broadcastClarification)
	router.POST("/api/v1/contests/:tid/clarifications/:cid/reply", h.replyClarification)
}

func (h *ContestHandler) list(c *gin.Context) {
	page, limit := parsePaging(c)
	items, total, err := h.repo.ListFiltered(c.Request.Context(), c.Query("q"), c.Query("rule"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list contests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": total, "page": page, "limit": limit},
	})
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
	// The invitation code is sensitive and is only exposed by this
	// owner/administrator-only endpoint.
	response := struct {
		*contest.Contest
		InvitationCode string `json:"invitationCode,omitempty"`
	}{Contest: item, InvitationCode: item.InvitationCode}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": response})
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
	var attendeeStatus *contest.AttendeeStatus
	if u, ok := h.currentUser(c); ok {
		attended, err = h.repo.Attended(c.Request.Context(), c.Param("tid"), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest attendance"})
			return
		}
		attendeeStatus, err = h.repo.StartIfAttended(c.Request.Context(), c.Param("tid"), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest status"})
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
		"data": gin.H{
			"contest": item, "problems": problems, "attended": attended,
			"status": attendeeStatus, "statuses": statuses,
		},
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
	Attended               bool                    `json:"attended"`
	RequiresInvitationCode bool                    `json:"requiresInvitationCode,omitempty"`
	Status                 *contest.AttendeeStatus `json:"status,omitempty"`
	// PrivateFiles holds contest materials, only filled in for owners,
	// administrators and participants after the contest started.
	PrivateFiles []filemeta.File `json:"privateFiles,omitempty"`
}

func (h *ContestHandler) unlock(c *gin.Context) {
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
	if time.Now().Before(item.EndAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contest is not ended"})
		return
	}
	updated, err := h.repo.Unlock(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to unlock scoreboard"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": updated})
}

func (h *ContestHandler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return
	}
	response := contestDetailResponse{
		Contest: item, RequiresInvitationCode: item.InvitationCode != "",
	}
	if u, ok := h.currentUser(c); ok {
		response.Attended, err = h.repo.Attended(c.Request.Context(), c.Param("tid"), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest attendance"})
			return
		}
		response.Status, err = h.repo.Status(c.Request.Context(), c.Param("tid"), u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest status"})
			return
		}
		if allowed, err := h.repo.CanReadPrivateFiles(c.Request.Context(), c.Param("tid"), u.ID); err == nil && (allowed || u.Priv&1 != 0) {
			if files, err := h.repo.PrivateFiles(c.Request.Context(), c.Param("tid")); err == nil {
				response.PrivateFiles = files
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": response,
	})
}

func (h *ContestHandler) attend(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
	}
	changed, err := h.repo.AttendContest(c.Request.Context(), c.Param("tid"), u.ID, request.Code)
	if err != nil {
		switch {
		case errors.Is(err, contest.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		case errors.Is(err, contest.ErrContestEnded):
			c.JSON(http.StatusBadRequest, gin.H{"error": "contest has ended"})
		case errors.Is(err, contest.ErrInvalidInvitationCode):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invitation code"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to attend contest"})
		}
		return
	}
	status, err := h.repo.Status(c.Request.Context(), c.Param("tid"), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"attended": true, "changed": changed, "status": status},
	})
}

func (h *ContestHandler) earlyEnd(c *gin.Context) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	status, err := h.repo.EarlyEnd(c.Request.Context(), c.Param("tid"), u.ID)
	if err != nil {
		switch {
		case errors.Is(err, contest.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		case errors.Is(err, contest.ErrContestNotAttended):
			c.JSON(http.StatusForbidden, gin.H{"error": "attend the contest first"})
		case errors.Is(err, contest.ErrContestEnded), errors.Is(err, contest.ErrContestNotLive):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to end contest"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": status})
}

// setScore stores the per-problem score, mirroring the upstream set_score
// operation on the contest management page.
func (h *ContestHandler) setScore(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	var request struct {
		PID   int64 `json:"pid"`
		Score int   `json:"score"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if request.PID <= 0 || request.Score < 0 || request.Score > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid score"})
		return
	}
	if err := h.repo.SetScore(c.Request.Context(), item.DocID.Hex(), request.PID, request.Score); err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "problem is not part of this contest"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update score"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ContestHandler) exportCode(c *gin.Context) {
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

// printAccess loads the contest and reports whether the caller may use the
// print station, mirroring the upstream ContestPrintHandler gate.
func (h *ContestHandler) printAccess(c *gin.Context) (*contest.Contest, *user.User, bool, bool) {
	u, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, nil, false, false
	}
	item, err := h.repo.Get(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
		return nil, nil, false, false
	}
	if !item.AllowPrint {
		c.JSON(http.StatusNotFound, gin.H{"error": "printing is disabled for this contest"})
		return nil, nil, false, false
	}
	isEditor := item.Owner == u.ID || u.Priv&1 != 0
	if !isEditor {
		attended, err := h.repo.Attended(c.Request.Context(), item.DocID.Hex(), u.ID)
		if err != nil || !attended {
			c.JSON(http.StatusForbidden, gin.H{"error": "attend the contest first"})
			return nil, nil, false, false
		}
	}
	return item, u, isEditor, true
}

func (h *ContestHandler) printTasks(c *gin.Context) {
	item, u, isEditor, ok := h.printAccess(c)
	if !ok {
		return
	}
	owner := u.ID
	if isEditor {
		owner = 0
	}
	tasks, err := h.repo.PrintTasks(c.Request.Context(), item.DocID.Hex(), owner)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load print tasks"})
		return
	}
	ids := make([]int64, 0, len(tasks))
	seen := map[int64]bool{}
	for _, task := range tasks {
		if task.Owner > 0 && !seen[task.Owner] {
			seen[task.Owner] = true
			ids = append(ids, task.Owner)
		}
	}
	users := []gin.H{}
	if len(ids) > 0 {
		items, err := h.users.FindByIDs(c.Request.Context(), ids)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load print tasks"})
			return
		}
		for _, account := range items {
			users = append(users, gin.H{
				"id": account.ID, "username": account.Uname,
				"displayName": account.DisplayName, "avatar": account.Avatar,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": tasks, "users": users, "isEditor": isEditor,
	}})
}

func (h *ContestHandler) addPrintTask(c *gin.Context) {
	item, u, _, ok := h.printAccess(c)
	if !ok {
		return
	}
	var request struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	if len(request.Content) > 1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is too large"})
		return
	}
	if item.EndAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contest has ended"})
		return
	}
	task, err := h.repo.AddPrintTask(c.Request.Context(), item.DocID.Hex(), u.ID, request.Title, request.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to queue print task"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": task})
}

func (h *ContestHandler) allocatePrintTask(c *gin.Context) {
	item, _, isEditor, ok := h.printAccess(c)
	if !ok {
		return
	}
	if !isEditor {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	task, err := h.repo.AllocatePrintTask(c.Request.Context(), item.DocID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to allocate print task"})
		return
	}
	if task == nil {
		c.Status(http.StatusNoContent)
		return
	}
	var owner *user.User
	if u, err := h.users.FindByID(c.Request.Context(), task.Owner); err == nil {
		owner = u
	}
	payload := gin.H{"task": task}
	if owner != nil {
		payload["user"] = gin.H{
			"id": owner.ID, "username": owner.Uname,
			"displayName": owner.DisplayName, "avatar": owner.Avatar,
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": payload})
}

func (h *ContestHandler) updatePrintTask(c *gin.Context) {
	item, _, isEditor, ok := h.printAccess(c)
	if !ok {
		return
	}
	if !isEditor {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var request struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if request.Status != contest.PrintPrinted && request.Status != contest.PrintPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	id, err := bson.ObjectIDFromHex(c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}
	if err := h.repo.UpdatePrintTask(c.Request.Context(), item.DocID.Hex(), id, request.Status); err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "print task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update print task"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ContestHandler) balloons(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	items, err := h.repo.Balloons(c.Request.Context(), item.DocID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load balloons"})
		return
	}
	colors := item.Balloon
	if colors == nil {
		colors = map[int64]contest.BalloonColor{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": items, "colors": colors,
	}})
}

func (h *ContestHandler) setBalloonColors(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	var request struct {
		Colors map[string]contest.BalloonColor `json:"colors"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	colors := make(map[int64]contest.BalloonColor, len(request.Colors))
	for raw, color := range request.Colors {
		pid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || pid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pid"})
			return
		}
		if strings.TrimSpace(color.Color) == "" || strings.TrimSpace(color.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "each problem needs a name and color"})
			return
		}
		colors[pid] = color
	}
	if err := h.repo.SetBalloonColors(c.Request.Context(), item.DocID.Hex(), colors); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save balloon colors"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ContestHandler) markBalloonSent(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	u, ok := h.currentUser(c)
	if !ok {
		return
	}
	bid, err := bson.ObjectIDFromHex(c.Param("bid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid balloon id"})
		return
	}
	if err := h.repo.MarkBalloonSent(c.Request.Context(), item.DocID.Hex(), bid, u.ID); err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "balloon not found or already sent"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update balloon"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ContestHandler) attendees(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	statuses, err := h.repo.Statuses(c.Request.Context(), item.DocID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load attendees"})
		return
	}
	ids := make([]int64, 0, len(statuses))
	for _, status := range statuses {
		ids = append(ids, status.UID)
	}
	users := []gin.H{}
	if len(ids) > 0 {
		items, err := h.users.FindByIDs(c.Request.Context(), ids)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load attendees"})
			return
		}
		for _, u := range items {
			users = append(users, gin.H{
				"id": u.ID, "username": u.Uname, "displayName": u.DisplayName, "avatar": u.Avatar,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": statuses, "users": users,
	}})
}

func (h *ContestHandler) addAttendees(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	var request struct {
		UIDs []int64 `json:"uids"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.UIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uids are required"})
		return
	}
	for _, uid := range request.UIDs {
		if uid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uid"})
			return
		}
		if _, err := h.users.FindByID(c.Request.Context(), uid); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if _, err := h.repo.Attend(c.Request.Context(), item.DocID.Hex(), uid); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to add attendee"})
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (h *ContestHandler) setAttendeeRank(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var request struct {
		Unrank bool `json:"unrank"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.repo.SetUnrank(c.Request.Context(), item.DocID.Hex(), uid, request.Unrank); err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "attendee not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update attendee"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ContestHandler) resumeAttendee(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	if !item.EndAt.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contest already ended"})
		return
	}
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if err := h.repo.Resume(c.Request.Context(), item.DocID.Hex(), uid); err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "attendee not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to resume attendee"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ContestHandler) cancelAttendee(c *gin.Context) {
	item, ok := h.requireEditor(c)
	if !ok {
		return
	}
	if !item.BeginAt.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contest already started"})
		return
	}
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if err := h.repo.CancelAttend(c.Request.Context(), item.DocID.Hex(), uid); err != nil {
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "attendee not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to cancel registration"})
		return
	}
	c.Status(http.StatusNoContent)
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

func (h *ContestHandler) broadcastClarification(c *gin.Context) {
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
		Subject int64  `json:"subject"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	created, err := h.repo.AddClarification(c.Request.Context(), c.Param("tid"), 0, request.Subject, request.Content)
	if err != nil {
		if errors.Is(err, contest.ErrInvalidClarificationSubject) || errors.Is(err, contest.ErrEmptyClarificationContent) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, contest.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to broadcast clarification"})
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
