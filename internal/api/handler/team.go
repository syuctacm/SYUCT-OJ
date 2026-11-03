package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type TeamHandler struct {
	teams    *contest.TeamRepository
	users    *user.Repository
	sessions *auth.SessionManager
}

func NewTeamHandler(teams *contest.TeamRepository, users *user.Repository, sessions *auth.SessionManager) *TeamHandler {
	return &TeamHandler{teams: teams, users: users, sessions: sessions}
}

func (h *TeamHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/contest/teams", h.snapshot)
	router.POST("/api/v1/contest/teams", h.create)
	router.PATCH("/api/v1/contest/teams/:vuid", h.rename)
	router.POST("/api/v1/contest/teams/:vuid/invite", h.invite)
	router.POST("/api/v1/contest/teams/:vuid/accept", h.accept)
	router.POST("/api/v1/contest/teams/:vuid/reject", h.reject)
	router.DELETE("/api/v1/contest/teams/:vuid/members/:uid", h.leave)
}

type teamUser struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

func (h *TeamHandler) snapshot(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	data, err := h.buildSnapshot(c, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load teams"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}

func (h *TeamHandler) buildSnapshot(c *gin.Context, uid int64) (gin.H, error) {
	mine, invites, err := h.teams.ListForUser(c.Request.Context(), uid)
	if err != nil {
		return nil, err
	}
	uidSet := make(map[int64]struct{})
	for _, team := range append(append([]contest.Team{}, mine...), invites...) {
		for _, member := range team.Members {
			if member > 0 {
				uidSet[member] = struct{}{}
			}
		}
		for _, invitee := range team.Invite {
			if invitee > 0 {
				uidSet[invitee] = struct{}{}
			}
		}
	}
	ids := make([]int64, 0, len(uidSet))
	for id := range uidSet {
		ids = append(ids, id)
	}
	users, err := h.users.FindByIDs(c.Request.Context(), ids)
	if err != nil {
		return nil, err
	}
	udict := make(map[string]teamUser, len(users))
	for _, item := range users {
		udict[strconv.FormatInt(item.ID, 10)] = teamUser{
			ID: item.ID, Username: item.Uname, DisplayName: item.DisplayName, Avatar: item.Avatar,
		}
	}
	return gin.H{"mine": mine, "invites": invites, "users": udict}, nil
}

func (h *TeamHandler) create(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if _, err := h.teams.Create(c.Request.Context(), uid, request.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondSnapshot(c, uid)
}

func (h *TeamHandler) rename(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	teamID, ok := parseTeamID(c)
	if !ok {
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if _, err := h.teams.Rename(c.Request.Context(), uid, teamID, request.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondSnapshot(c, uid)
}

func (h *TeamHandler) invite(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	teamID, ok := parseTeamID(c)
	if !ok {
		return
	}
	var request struct {
		User string `json:"user"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	target, err := strconv.ParseInt(strings.TrimSpace(request.User), 10, 64)
	if err != nil || target <= 0 {
		found, lookupErr := h.users.FindByLogin(c.Request.Context(), request.User)
		if lookupErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user not found"})
			return
		}
		target = found.ID
	}
	if _, err := h.teams.Invite(c.Request.Context(), uid, target, teamID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondSnapshot(c, uid)
}

func (h *TeamHandler) accept(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	teamID, ok := parseTeamID(c)
	if !ok {
		return
	}
	if _, err := h.teams.Accept(c.Request.Context(), uid, teamID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondSnapshot(c, uid)
}

func (h *TeamHandler) reject(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	teamID, ok := parseTeamID(c)
	if !ok {
		return
	}
	if err := h.teams.Reject(c.Request.Context(), uid, teamID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondSnapshot(c, uid)
}

func (h *TeamHandler) leave(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	teamID, ok := parseTeamID(c)
	if !ok {
		return
	}
	target, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || target <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member"})
		return
	}
	if _, err := h.teams.Leave(c.Request.Context(), uid, target, teamID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondSnapshot(c, uid)
}

func (h *TeamHandler) respondSnapshot(c *gin.Context, uid int64) {
	data, err := h.buildSnapshot(c, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load teams"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}

func parseTeamID(c *gin.Context) (int64, bool) {
	teamID, err := strconv.ParseInt(c.Param("vuid"), 10, 64)
	if err != nil || teamID >= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return 0, false
	}
	return teamID, true
}

func (h *TeamHandler) currentUser(c *gin.Context) (int64, bool) {
	token := auth.Token(c)
	session, ok := h.sessions.GetContext(c.Request.Context(), token)
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return 0, false
	}
	return session.UserID, true
}
