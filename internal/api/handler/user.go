package handler

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/user"
)

type UserHandler struct {
	users    *user.Repository
	sessions *auth.SessionManager
	problems *problem.Repository
}

func NewUserHandler(users *user.Repository, sessions *auth.SessionManager, problems *problem.Repository) *UserHandler {
	return &UserHandler{users: users, sessions: sessions, problems: problems}
}

func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/users", h.list)
	router.GET("/api/v1/groups", h.groups)
	router.GET("/api/v1/users/:uid", h.get)
	router.GET("/api/v1/users/:uid/activity", h.activity)
	router.GET("/api/v1/ranking", h.ranking)
	router.PATCH("/api/v1/user/settings", h.updateSettings)
	router.GET("/api/v1/user/security/sessions", h.securitySessions)
	router.DELETE("/api/v1/user/security/sessions", h.securityRevokeAll)
	router.DELETE("/api/v1/user/security/sessions/:digest", h.securityRevokeSession)
	router.POST("/api/v1/user/security/password", h.securityChangePassword)
}

type userChoice struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

type groupChoice struct {
	Name string  `json:"name"`
	UIDs []int64 `json:"uids"`
}

func (h *UserHandler) list(c *gin.Context) {
	rawIDs := c.Query("auto")
	if rawIDs == "" {
		rawIDs = c.Query("ids")
	}
	var items []user.User
	var err error
	if rawIDs != "" {
		ids := make([]int64, 0)
		for _, value := range strings.Split(rawIDs, ",") {
			id, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if parseErr == nil && id > 0 {
				ids = append(ids, id)
			}
		}
		items, err = h.users.FindByIDs(c.Request.Context(), ids)
	} else {
		items, err = h.users.Search(c.Request.Context(), c.Query("search"), 20)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to search users"})
		return
	}
	choices := make([]userChoice, 0, len(items))
	for _, item := range items {
		choices = append(choices, userChoice{
			ID: item.ID, Username: item.Uname, DisplayName: item.DisplayName, Avatar: item.Avatar,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": choices}})
}

func (h *UserHandler) groups(c *gin.Context) {
	rawNames := c.Query("names")
	var items []user.Group
	var err error
	if rawNames != "" {
		items, err = h.users.FindGroupsByNames(c.Request.Context(), strings.Split(rawNames, ","))
	} else {
		items, err = h.users.SearchGroups(c.Request.Context(), c.Query("search"), 20)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to search groups"})
		return
	}
	choices := make([]groupChoice, 0, len(items))
	for _, item := range items {
		choices = append(choices, groupChoice{Name: item.Name, UIDs: item.UIDs})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": choices}})
}

func (h *UserHandler) activity(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	activity, err := h.users.Activity(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load activity"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": activity})
}

func (h *UserHandler) get(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	u, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	submit, accepted, err := h.users.Statistics(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load user statistics"})
		return
	}
	acceptedProblems, err := h.users.AcceptedProblems(c.Request.Context(), uid, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load accepted problems"})
		return
	}
	viewer, isViewer := h.currentUser(c)
	canSeePrivate := isViewer && viewer == uid
	if isViewer && !canSeePrivate {
		if account, err := h.users.FindByID(c.Request.Context(), viewer); err == nil && account.Priv&1 != 0 {
			canSeePrivate = true
		}
	}
	rp, level, err := h.users.DomainStats(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load user stats"})
		return
	}
	rank, err := h.users.Rank(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load user rank"})
		return
	}
	var nSolution, nLiked int64
	if h.problems != nil {
		nSolution, nLiked, err = h.problems.SolutionStats(c.Request.Context(), uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load solution statistics"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": profilePayload(u, canSeePrivate, submit, accepted, rp, level, rank, acceptedProblems, nSolution, nLiked),
	})
}

// profilePayload keeps upstream privacy flags: school and student id are only
// exposed to the owner and to system administrators.
func profilePayload(u *user.User, canSeePrivate bool, submit, accepted int64, rp float64, level, rank int, acceptedProblems []user.ProblemBrief, nSolution, nLiked int64) gin.H {
	payload := gin.H{
		"id": u.ID, "username": u.Uname, "displayName": u.DisplayName,
		"avatar": u.Avatar, "regAt": u.RegAt, "loginAt": u.LoginAt,
		"bio": u.Bio, "qq": u.QQ, "gender": u.Gender, "backgroundImage": u.BackgroundImage,
		"nSubmit": submit, "nAccept": accepted, "rp": rp, "level": level, "rank": rank,
		"nSolution": nSolution, "nLiked": nLiked,
		"acceptedProblems": acceptedProblems,
	}
	if canSeePrivate {
		payload["school"] = u.School
		payload["studentId"] = u.StudentID
	}
	return payload
}

var (
	avatarGitHubPattern = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	avatarQQPattern     = regexp.MustCompile(`^[1-9][0-9]{4,}$`)
	avatarEmailPattern  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	backgroundPattern   = regexp.MustCompile(`^[^\s"'();]+$`)
)

// validAvatar mirrors the upstream avatar.validate() provider checks.
func validAvatar(value string) bool {
	if value == "" {
		return true
	}
	switch {
	case strings.HasPrefix(value, "url:"):
		return true
	case strings.HasPrefix(value, "github:"):
		return avatarGitHubPattern.MatchString(value[len("github:"):])
	case strings.HasPrefix(value, "qq:"):
		return avatarQQPattern.MatchString(value[len("qq:"):])
	case strings.HasPrefix(value, "gravatar:"):
		return avatarEmailPattern.MatchString(value[len("gravatar:"):])
	default:
		return false
	}
}

func validBackground(value string) bool {
	if value == "" {
		return true
	}
	if !backgroundPattern.MatchString(value) {
		return false
	}
	return strings.HasPrefix(value, "/") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func (h *UserHandler) updateSettings(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var input struct {
		Avatar          *string `json:"avatar"`
		QQ              *string `json:"qq"`
		Gender          *int64  `json:"gender"`
		Bio             *string `json:"bio"`
		School          *string `json:"school"`
		StudentID       *string `json:"studentId"`
		BackgroundImage *string `json:"backgroundImage"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	set := bson.M{}
	if input.Avatar != nil {
		value := strings.TrimSpace(*input.Avatar)
		if !validAvatar(value) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid avatar"})
			return
		}
		set["avatar"] = value
	}
	if input.QQ != nil {
		value := strings.TrimSpace(*input.QQ)
		if len(value) > 32 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "qq is too long"})
			return
		}
		set["qq"] = value
	}
	if input.Gender != nil {
		if *input.Gender < 0 || *input.Gender > 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gender"})
			return
		}
		set["gender"] = *input.Gender
	}
	if input.Bio != nil {
		if len(*input.Bio) > 10000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bio is too long"})
			return
		}
		set["bio"] = *input.Bio
	}
	if input.School != nil {
		value := strings.TrimSpace(*input.School)
		if len(value) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "school is too long"})
			return
		}
		set["school"] = value
	}
	if input.StudentID != nil {
		value := strings.TrimSpace(*input.StudentID)
		if len(value) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "student id is too long"})
			return
		}
		set["studentId"] = value
	}
	if input.BackgroundImage != nil {
		value := strings.TrimSpace(*input.BackgroundImage)
		if len(value) > 512 || !validBackground(value) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid background image"})
			return
		}
		set["backgroundImage"] = value
	}
	if err := h.users.UpdateSettings(c.Request.Context(), uid, set); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update settings"})
		return
	}
	updated, err := h.users.FindByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reload user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"id": updated.ID, "username": updated.Uname, "displayName": updated.DisplayName,
		"avatar": updated.Avatar, "bio": updated.Bio, "qq": updated.QQ,
		"gender": updated.Gender, "school": updated.School,
		"studentId": updated.StudentID, "backgroundImage": updated.BackgroundImage,
	}})
}

func (h *UserHandler) currentUser(c *gin.Context) (int64, bool) {
	if h.sessions == nil {
		return 0, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return 0, false
	}
	return session.UserID, true
}

func (h *UserHandler) ranking(c *gin.Context) {
	page, limit := parsePaging(c)
	entries, total, err := h.users.Ranking(c.Request.Context(), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load ranking"})
		return
	}
	uids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		uids = append(uids, entry.UID)
	}
	docs, err := h.users.FindByIDs(c.Request.Context(), uids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load ranking users"})
		return
	}
	byID := make(map[int64]user.User, len(docs))
	for _, doc := range docs {
		byID[doc.ID] = doc
	}
	items := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		doc, ok := byID[entry.UID]
		if !ok {
			continue
		}
		_, accepted, err := h.users.Statistics(c.Request.Context(), entry.UID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load ranking statistics"})
			return
		}
		items = append(items, gin.H{
			"id": doc.ID, "username": doc.Uname, "displayName": doc.DisplayName,
			"avatar": doc.Avatar, "bio": doc.Bio,
			"rp": entry.RP, "level": entry.Level, "nAccept": accepted,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"items": items, "total": total, "page": page, "limit": limit},
	})
}
