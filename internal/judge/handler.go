package judge

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
)

type Handler struct {
	username string
	password string
	sessions *auth.SessionManager
	store    *FileStore
	hub      *Hub
}

func NewHandler(username, password string, sessions *auth.SessionManager, store *FileStore, hub *Hub) *Handler {
	return &Handler{username: username, password: password, sessions: sessions, store: store, hub: hub}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/login", h.login)

	authorized := router.Group("/")
	authorized.Use(h.requireSession)
	authorized.GET("/judge/files", h.filesCheck)
	authorized.POST("/judge/files", h.filesLinks)
	authorized.POST("/d/:domainId/judge/files", h.problemFileLinks)
	authorized.POST("/judge/upload", h.upload)
	authorized.GET("/judge/files/submission/:id/code", h.submissionCode)
	authorized.GET("/judge/files/problem/:domainId/:pid/:filename", h.problemFile)
	authorized.GET("/judge/conn", h.websocket)
}

type loginRequest struct {
	Username string `form:"uname" json:"uname"`
	Password string `form:"password" json:"password"`
}

func (h *Handler) login(c *gin.Context) {
	var request loginRequest
	if strings.HasPrefix(c.ContentType(), "application/json") {
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
	} else if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if request.Username != h.username || request.Password != h.password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := h.sessions.Issue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "sid",
		Value:    token,
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) requireSession(c *gin.Context) {
	token := ""
	if cookie, err := c.Cookie("sid"); err == nil {
		token = cookie
	}
	if authorization := c.GetHeader("Authorization"); strings.HasPrefix(authorization, "Bearer ") {
		token = strings.TrimPrefix(authorization, "Bearer ")
	}
	if !h.sessions.Validate(token) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Set("session_token", token)
	c.Next()
}

func (h *Handler) filesCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) filesLinks(c *gin.Context) {
	var request struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": "/judge/files/submission/" + request.ID + "/code"})
}

func (h *Handler) problemFileLinks(c *gin.Context) {
	var request struct {
		PID   int      `json:"pid"`
		Files []string `json:"files"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.PID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	links := make(map[string]string, len(request.Files))
	for _, name := range request.Files {
		links[name] = "/judge/files/problem/" + c.Param("domainId") + "/" + strconv.Itoa(request.PID) + "/" + name
	}
	c.JSON(http.StatusOK, gin.H{"links": links})
}

func (h *Handler) upload(c *gin.Context) {
	rid := c.PostForm("rid")
	name := c.PostForm("name")
	if rid == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rid and name are required"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	source, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to open file"})
		return
	}
	defer source.Close()
	if err := h.store.SaveUpload(rid, name, source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": 1})
}

func (h *Handler) submissionCode(c *gin.Context) {
	h.serveFile(c, func() (io.ReadCloser, error) {
		return h.store.OpenSubmission(c.Param("id"))
	})
}

func (h *Handler) problemFile(c *gin.Context) {
	h.serveFile(c, func() (io.ReadCloser, error) {
		pid := c.Param("pid")
		return h.store.OpenProblemFile(c.Param("domainId"), pid, c.Param("filename"))
	})
}

func (h *Handler) serveFile(c *gin.Context, open func() (io.ReadCloser, error)) {
	reader, err := open()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	defer reader.Close()
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

func (h *Handler) websocket(c *gin.Context) {
	h.hub.ServeHTTP(c.Writer, c.Request)
}
