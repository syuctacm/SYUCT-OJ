package handler

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type SubmissionHandler struct {
	repo     *submission.Repository
	sessions *auth.SessionManager
	contests *contest.Repository
	users    *user.Repository
	hub      *submission.RecordHub
	store    *judge.FileStore
}

func NewSubmissionHandler(repo *submission.Repository, sessions *auth.SessionManager, contests *contest.Repository, users *user.Repository) *SubmissionHandler {
	return &SubmissionHandler{repo: repo, sessions: sessions, contests: contests, users: users}
}

// Repository exposes the submission store for handlers that need read-only
// access to records (the problem statistics page).
func (h *SubmissionHandler) SetHub(hub *submission.RecordHub) {
	h.hub = hub
}

func (h *SubmissionHandler) SetFileStore(store *judge.FileStore) {
	h.store = store
}

func (h *SubmissionHandler) Repository() *submission.Repository {
	if h == nil {
		return nil
	}
	return h.repo
}

func (h *SubmissionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/submissions", h.create)
	router.GET("/api/v1/submissions", h.list)
	if h.hub != nil {
		router.GET("/api/v1/submissions/events", h.events)
	}
	router.GET("/api/v1/submissions/:rid", h.get)
	router.GET("/api/v1/submissions/:rid/code", h.downloadCode)
	router.GET("/api/v1/submissions/:rid/hack", h.downloadHack)
	router.POST("/api/v1/problems/:pid/hack/:rid", h.createHack)
	router.POST("/api/v1/submissions/:rid/rejudge", h.rejudge)
	router.POST("/api/v1/submissions/:rid/cancel", h.cancel)
}

func (h *SubmissionHandler) create(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		PID       string `json:"pid"`
		Lang      string `json:"lang"`
		Code      string `json:"code"`
		ContestID string `json:"contestId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.PID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pid, lang and code are required"})
		return
	}
	if int64(len(request.Code)) > configuredLimitBytes(c, settingLimitCodeLength, defaultLimitCodeLength) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is too long"})
		return
	}
	recordUID := uid
	if request.ContestID != "" && h.contests != nil {
		item, err := h.contests.Get(c.Request.Context(), request.ContestID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "contest not found"})
			return
		}
		status, err := h.contests.StatusForUser(c.Request.Context(), request.ContestID, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest status"})
			return
		}
		if status == nil || status.Attend == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "attend the contest first"})
			return
		}
		recordUID = status.UID
		now := time.Now()
		deadline := item.EndAt
		if status.EndAt != nil && status.EndAt.Before(deadline) {
			deadline = *status.EndAt
		}
		if item.Duration > 0 && status.StartAt != nil {
			personalDeadline := status.StartAt.Add(time.Duration(item.Duration) * time.Hour)
			if personalDeadline.Before(deadline) {
				deadline = personalDeadline
			}
		}
		if now.Before(item.BeginAt) || !now.Before(deadline) {
			c.JSON(http.StatusForbidden, gin.H{"error": "contest is not running"})
			return
		}
	}
	record, err := h.repo.Create(c.Request.Context(), recordUID, request.PID, request.Lang, request.Code, request.ContestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "success", "data": record})
}

func (h *SubmissionHandler) list(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	page, limit := parsePaging(c, settingPaginationRecord)
	value := strings.TrimSpace(c.Query("uidOrName"))
	var filterUID int64
	if value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed != 0 {
			filterUID = parsed
		} else if h.users != nil {
			found, err := h.users.FindByLogin(c.Request.Context(), value)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": submission.ListResult{Items: []submission.Record{}, Page: page, Limit: limit}})
				return
			}
			filterUID = found.ID
		}
	}
	if tid := strings.TrimSpace(c.Query("tid")); tid != "" && h.contests != nil {
		if status, err := h.contests.StatusForUser(c.Request.Context(), tid, uid); err == nil && status != nil && status.UID < 0 {
			// A team's submissions are stored under its negative vuser UID. The
			// contest sidebar links to the signed-in user's own records, so map
			// that self filter to the team participant.
			if filterUID == uid || filterUID == 0 && value == "" {
				filterUID = status.UID
			}
		}
	}
	var status *int
	if value := c.Query("status"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			status = &parsed
		}
	}
	result, err := h.repo.List(c.Request.Context(), submission.ListFilter{
		Page:      page,
		Limit:     limit,
		UID:       filterUID,
		PID:       c.Query("pid"),
		ContestID: c.Query("tid"),
		Lang:      c.Query("lang"),
		Status:    status,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list submissions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *SubmissionHandler) events(c *gin.Context) {
	if _, ok := h.currentUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.hub.Serve(c.Writer, c.Request); err != nil {
		return
	}
}

func (h *SubmissionHandler) get(c *gin.Context) {
	record, err := h.repo.Get(c.Request.Context(), c.Param("rid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}
	if uid, ok := h.currentUser(c); !ok || !h.canReadCode(c, uid, record) {
		record.Code = ""
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": record})
}

func (h *SubmissionHandler) canReadCode(c *gin.Context, uid int64, record *submission.Record) bool {
	if uid == record.UID {
		return true
	}
	u, err := h.users.FindByID(c.Request.Context(), uid)
	if err == nil && u.Priv&1 != 0 {
		return true
	}
	if record.Contest == nil || record.UID >= 0 || h.contests == nil {
		return false
	}
	status, err := h.contests.Status(c.Request.Context(), record.Contest.Hex(), record.UID)
	return err == nil && status != nil && containsUserID(status.Members, uid)
}

func containsUserID(values []int64, uid int64) bool {
	for _, value := range values {
		if value == uid {
			return true
		}
	}
	return false
}

func (h *SubmissionHandler) requireAdmin(c *gin.Context) (*user.User, bool) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	u, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	if u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return u, true
}

func (h *SubmissionHandler) rejudge(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}
	record, err := h.repo.RejudgeRecord(c.Request.Context(), c.Param("rid"))
	if err != nil {
		if errors.Is(err, submission.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": record})
}

func (h *SubmissionHandler) cancel(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}
	record, err := h.repo.CancelRecord(c.Request.Context(), c.Param("rid"))
	if err != nil {
		if errors.Is(err, submission.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": record})
}

func (h *SubmissionHandler) downloadCode(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	record, err := h.repo.Get(c.Request.Context(), c.Param("rid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}
	if !h.canReadCode(c, uid, record) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	ext := ".txt"
	if record.Lang != "" {
		ext = "." + strings.ReplaceAll(record.Lang, " ", "_")
	}
	filename := "submission-" + record.ID.Hex() + ext
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	_, _ = c.Writer.WriteString(record.Code)
}

func (h *SubmissionHandler) createHack(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hack storage is unavailable"})
		return
	}
	input, autoOrganize, err := readHackInput(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if autoOrganize {
		input = organizeHackInput(input)
	}
	if len(input) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hack input is required"})
		return
	}
	contestID := strings.TrimSpace(c.Query("tid"))
	if contestID != "" {
		if h.contests == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "contest service is unavailable"})
			return
		}
		item, err := h.contests.Get(c.Request.Context(), contestID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
			return
		}
		if item.Rule != "codeforces" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "this contest type does not allow hacks"})
			return
		}
		status, err := h.contests.StatusForUser(c.Request.Context(), contestID, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load contest status"})
			return
		}
		if status == nil || status.Attend == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "attend the contest first"})
			return
		}
		if !contest.IsOngoing(item, status, time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{"error": "contest is not running"})
			return
		}
		uid = status.UID
	}
	record, err := h.repo.CreateHack(c.Request.Context(), uid, c.Param("pid"), c.Param("rid"), input, contestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "success", "data": record})
}

func readHackInput(c *gin.Context) ([]byte, bool, error) {
	const maxInput = 2 << 20
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(maxInput + 1024); err != nil {
			return nil, false, fmt.Errorf("invalid multipart form")
		}
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		autoOrganize := c.Request.FormValue("autoOrganizeInput") == "on" || c.Request.FormValue("autoOrganizeInput") == "true"
		file, header, err := c.Request.FormFile("file")
		if err == nil {
			defer file.Close()
			if header.Size > maxInput {
				return nil, autoOrganize, fmt.Errorf("hack input is too large")
			}
			input, err := io.ReadAll(io.LimitReader(file, maxInput+1))
			if err != nil {
				return nil, autoOrganize, err
			}
			if len(input) > maxInput {
				return nil, autoOrganize, fmt.Errorf("hack input is too large")
			}
			return input, autoOrganize, nil
		}
		input := []byte(c.Request.FormValue("input"))
		if len(input) > maxInput {
			return nil, autoOrganize, fmt.Errorf("hack input is too large")
		}
		return input, autoOrganize, nil
	}
	var request struct {
		Input             string `json:"input"`
		AutoOrganizeInput bool   `json:"autoOrganizeInput"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		return nil, false, fmt.Errorf("invalid request")
	}
	if len(request.Input) > maxInput {
		return nil, request.AutoOrganizeInput, fmt.Errorf("hack input is too large")
	}
	return []byte(request.Input), request.AutoOrganizeInput, nil
}

func organizeHackInput(input []byte) []byte {
	text := strings.ReplaceAll(string(input), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return []byte(strings.Join(lines, "\n"))
}

func (h *SubmissionHandler) downloadHack(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	record, err := h.repo.Get(c.Request.Context(), c.Param("rid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}
	if !h.canReadCode(c, uid, record) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	reader, err := h.repo.OpenHackInput(c.Request.Context(), record.ID.Hex())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "hack input not found"})
		return
	}
	defer reader.Close()
	filename := "hack-" + record.ID.Hex() + ".in"
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.Copy(c.Writer, io.LimitReader(reader, 2<<20))
}

func (h *SubmissionHandler) currentUser(c *gin.Context) (int64, bool) {
	token := auth.Token(c)
	session, ok := h.sessions.GetContext(c.Request.Context(), token)
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return 0, false
	}
	return session.UserID, true
}
