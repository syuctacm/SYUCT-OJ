package handler

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/judge"
)

const (
	maxAvatarFileSize = 8 << 20
	maxAvatarUpload   = maxAvatarFileSize + (1 << 20)
	avatarFilePrefix  = ".avatar"
)

var avatarExtensions = []string{".jpg", ".jpeg", ".png"}

type userAvatarRepository interface {
	UpdateSettings(context.Context, int64, bson.M) error
}

type UserAvatarHandler struct {
	repo     userAvatarRepository
	store    *judge.FileStore
	sessions *auth.SessionManager
}

func NewUserAvatarHandler(repo userAvatarRepository, store *judge.FileStore, sessions *auth.SessionManager) *UserAvatarHandler {
	return &UserAvatarHandler{repo: repo, store: store, sessions: sessions}
}

func (h *UserAvatarHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/api/v1/user/avatar", h.upload)
	router.GET("/api/v1/users/:uid/avatar", h.download)
}

func (h *UserAvatarHandler) upload(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file storage is unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarUpload)
	if err := c.Request.ParseMultipartForm(maxAvatarFileSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart upload"})
		return
	}
	headers := c.Request.MultipartForm.File["file"]
	if len(headers) == 0 {
		headers = c.Request.MultipartForm.File["avatar"]
	}
	if len(headers) != 1 || headers[0].Size > maxAvatarFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("avatar must be one JPG or PNG file smaller than %d MiB", maxAvatarFileSize>>20)})
		return
	}
	header := headers[0]
	originalName := path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	extension := strings.ToLower(path.Ext(originalName))
	if !validAvatarExtension(extension) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar must be a JPG or PNG image"})
		return
	}
	source, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to open avatar"})
		return
	}
	defer source.Close()
	_, format, err := image.DecodeConfig(source)
	if err != nil || (format != "jpeg" && format != "png") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar must be a valid JPG or PNG image"})
		return
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read avatar"})
		return
	}
	name := avatarFilePrefix + avatarExtensionForFormat(format)
	stored, err := h.store.PutUserFile(uid, name, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save avatar"})
		return
	}
	for _, candidate := range avatarExtensions {
		candidateName := avatarFilePrefix + candidate
		if candidateName != name {
			_ = h.store.DeleteUserFile(uid, candidateName)
		}
	}
	avatarURL := fmt.Sprintf("/api/v1/users/%d/avatar?v=%s", uid, url.QueryEscape(stored.ETag))
	if err := h.repo.UpdateSettings(c.Request.Context(), uid, bson.M{"avatar": "url:" + avatarURL}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update avatar"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"avatar": avatarURL}})
}

func (h *UserAvatarHandler) download(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file storage is unavailable"})
		return
	}
	for _, extension := range avatarExtensions {
		reader, err := h.store.OpenUserFile(uid, avatarFilePrefix+extension)
		if err != nil {
			continue
		}
		defer reader.Close()
		c.Header("Content-Type", mime.TypeByExtension(extension))
		c.Header("Content-Disposition", "inline")
		c.Header("Cache-Control", "public, max-age=3600")
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, reader)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "avatar not found"})
}

func (h *UserAvatarHandler) requireUser(c *gin.Context) (int64, bool) {
	if h.sessions == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	return session.UserID, true
}

func validAvatarExtension(extension string) bool {
	for _, candidate := range avatarExtensions {
		if extension == candidate {
			return true
		}
	}
	return false
}

func avatarExtensionForFormat(format string) string {
	if format == "jpeg" {
		return ".jpg"
	}
	return ".png"
}
