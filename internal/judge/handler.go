package judge

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

const problemFileKindTestdata = "testdata"

type Handler struct {
	username string
	password string
	sessions *auth.SessionManager
	store    *FileStore
	hub      *Hub
	db       *mongo.Database
	problems *problem.Repository
}

func NewHandler(username, password string, sessions *auth.SessionManager, store *FileStore, hub *Hub) *Handler {
	return &Handler{username: username, password: password, sessions: sessions, store: store, hub: hub}
}

func (h *Handler) Store() *FileStore {
	return h.store
}

// EnableGenerateUpload lets /judge/upload persist generated testdata into the
// problem referenced by an active type=generate task.
func (h *Handler) EnableGenerateUpload(db *mongo.Database, problems *problem.Repository) {
	h.db = db
	h.problems = problems
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
	token, err := h.sessions.IssueContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "sid",
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.sessions.TTL().Seconds()),
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
	if !h.sessions.ValidateContext(c.Request.Context(), token) {
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

type judgeUpload struct {
	rid  string
	name string
	file *os.File
}

func readJudgeUpload(c *gin.Context) (*judgeUpload, error) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return nil, err
	}
	upload := &judgeUpload{}
	cleanup := func() {
		if upload.file != nil {
			name := upload.file.Name()
			_ = upload.file.Close()
			_ = os.Remove(name)
			upload.file = nil
		}
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			cleanup()
			return nil, err
		}
		switch part.FormName() {
		case "rid", "name":
			value, err := io.ReadAll(io.LimitReader(part, 1<<20))
			if err != nil {
				_ = part.Close()
				cleanup()
				return nil, err
			}
			if part.FormName() == "rid" {
				upload.rid = string(value)
			} else {
				upload.name = string(value)
			}
		case "file":
			if upload.file != nil {
				_ = part.Close()
				cleanup()
				return nil, fmt.Errorf("duplicate file")
			}
			upload.file, err = os.CreateTemp("", "syuct-oj-judge-upload-*")
			if err == nil {
				_, err = io.Copy(upload.file, part)
			}
			if err != nil {
				_ = part.Close()
				cleanup()
				return nil, err
			}
			if _, err := upload.file.Seek(0, io.SeekStart); err != nil {
				_ = part.Close()
				cleanup()
				return nil, err
			}
		}
		_ = part.Close()
	}
	if upload.rid == "" || upload.name == "" || upload.file == nil {
		cleanup()
		return nil, fmt.Errorf("rid, name and file are required")
	}
	return upload, nil
}

func (h *Handler) upload(c *gin.Context) {
	upload, err := readJudgeUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer func() {
		name := upload.file.Name()
		_ = upload.file.Close()
		_ = os.Remove(name)
	}()
	rid, name, source := upload.rid, upload.name, upload.file

	if h.db != nil && h.problems != nil {
		ridObjectID, err := bson.ObjectIDFromHex(rid)
		if err == nil {
			var task struct {
				Type string `bson:"type"`
				PID  int64  `bson:"pid"`
			}
			if err := h.db.Collection("task").FindOne(c.Request.Context(), bson.M{"rid": ridObjectID}).Decode(&task); err == nil && task.Type == "generate" && task.PID > 0 {
				name = path.Base(strings.ReplaceAll(name, "\\", "/"))
				if name == "" || name == "." || name == ".." {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
					return
				}
				stored, err := h.store.PutProblemFile(problem.DomainSystem, strconv.FormatInt(task.PID, 10), name, source)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save generated testdata"})
					return
				}
				pdoc, err := h.problems.GetForEditor(c.Request.Context(), strconv.FormatInt(task.PID, 10))
				if err != nil {
					_ = h.store.DeleteProblemFile(problem.DomainSystem, strconv.FormatInt(task.PID, 10), name)
					c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
					return
				}
				generated := problem.ProblemFile{
					ID:   fmt.Sprintf("problem/%s/%d/testdata/%s", problem.DomainSystem, task.PID, name),
					Name: name, Size: stored.Size, ETag: stored.ETag, LastModified: stored.LastModified,
				}
				if err := h.problems.UpsertFile(c.Request.Context(), pdoc.DocID, problemFileKindTestdata, generated); err != nil {
					_ = h.store.DeleteProblemFile(problem.DomainSystem, strconv.FormatInt(task.PID, 10), name)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update generated testdata metadata"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"ok": 1})
				return
			}
		}
	}

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
