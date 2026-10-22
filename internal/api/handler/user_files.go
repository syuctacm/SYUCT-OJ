package handler

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/filemeta"
	"github.com/syuctacm/syuct-oj/internal/judge"
)

// Defaults of the upstream `limit.user_files` and `limit.user_files_size`
// settings.
const (
	maxUserFiles     = 100
	maxUserFileSize  = 128 << 20
	maxUserMultipart = 32 << 20
)

type userFileRepository interface {
	Files(context.Context, int64) ([]filemeta.File, error)
	UpsertFile(context.Context, int64, filemeta.File) error
	DeleteFiles(context.Context, int64, []string) error
}

type UserFileHandler struct {
	repo     userFileRepository
	store    *judge.FileStore
	sessions *auth.SessionManager
}

func NewUserFileHandler(repo userFileRepository, store *judge.FileStore, sessions *auth.SessionManager) *UserFileHandler {
	return &UserFileHandler{repo: repo, store: store, sessions: sessions}
}

func (h *UserFileHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/user/files", h.list)
	router.POST("/api/v1/user/files", h.upload)
	router.DELETE("/api/v1/user/files", h.delete)
	router.GET("/api/v1/user/files/:name", h.downloadOwn)
	router.GET("/api/v1/users/:uid/files/:name", h.downloadPublic)
}

func (h *UserFileHandler) list(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	files, err := h.repo.Files(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": userFileViews(uid, files), "total": len(files),
	}})
}

func (h *UserFileHandler) upload(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file storage is unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUserFileSize)
	if err := c.Request.ParseMultipartForm(maxUserMultipart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart upload"})
		return
	}
	headers := c.Request.MultipartForm.File["files"]
	if len(headers) == 0 {
		headers = c.Request.MultipartForm.File["file"]
	}
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one file is required"})
		return
	}
	existing, err := h.repo.Files(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load files"})
		return
	}
	if len(existing)+len(headers) > maxUserFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files (max %d)", maxUserFiles)})
		return
	}
	var total int64
	names := map[string]bool{}
	for _, item := range existing {
		total += item.Size
		names[item.Name] = true
	}
	for _, header := range headers {
		total += header.Size
	}
	if total > maxUserFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "files are too large"})
		return
	}
	for _, header := range headers {
		name, err := sanitizeAssetName(path.Base(strings.ReplaceAll(header.Filename, "\\", "/")))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if names[name] {
			c.JSON(http.StatusConflict, gin.H{"error": "file already exists"})
			return
		}
		source, err := header.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to open uploaded file"})
			return
		}
		stored, err := h.store.PutUserFile(uid, name, source)
		_ = source.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save uploaded file"})
			return
		}
		file := filemeta.File{ID: name, Name: name, Size: stored.Size, ETag: stored.ETag, LastModified: stored.LastModified}
		if err := h.repo.UpsertFile(c.Request.Context(), uid, file); err != nil {
			_ = h.store.DeleteUserFile(uid, name)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update file metadata"})
			return
		}
		names[name] = true
	}
	files, err := h.repo.Files(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reload files"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": userFileViews(uid, files), "total": len(files),
	}})
}

func (h *UserFileHandler) delete(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	var request struct {
		Names []string `json:"names"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "names are required"})
		return
	}
	names := make([]string, 0, len(request.Names))
	for _, value := range request.Names {
		name, err := sanitizeAssetName(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		names = append(names, name)
	}
	if err := h.repo.DeleteFiles(c.Request.Context(), uid, names); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update file metadata"})
		return
	}
	for _, name := range names {
		if h.store != nil {
			_ = h.store.DeleteUserFile(uid, name)
		}
	}
	c.Status(http.StatusNoContent)
}

func (h *UserFileHandler) downloadOwn(c *gin.Context) {
	uid, ok := h.requireUser(c)
	if !ok {
		return
	}
	h.serve(c, uid, c.Param("name"))
}

// downloadPublic matches the upstream fs_download handler: user files are
// addressable by owner id and can be embedded from public pages such as
// discussions without requiring a browser session.
func (h *UserFileHandler) downloadPublic(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	h.serve(c, uid, c.Param("name"))
}

func (h *UserFileHandler) serve(c *gin.Context, uid int64, rawName string) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file storage is unavailable"})
		return
	}
	name, err := sanitizeAssetName(rawName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	files, err := h.repo.Files(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load files"})
		return
	}
	found := false
	for _, file := range files {
		if file.Name == name {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	reader, err := h.store.OpenUserFile(uid, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	defer reader.Close()
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", path.Base(name)))
	c.Header("Cache-Control", "public")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

func (h *UserFileHandler) requireUser(c *gin.Context) (int64, bool) {
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

func userFileViews(uid int64, files []filemeta.File) []activityFile {
	items := make([]activityFile, 0, len(files))
	for _, file := range files {
		items = append(items, activityFile{
			ID: file.ID, Name: file.Name, Size: file.Size, ETag: file.ETag, LastModified: file.LastModified,
			DownloadURL: fmt.Sprintf("/api/v1/users/%d/files/%s", uid, url.PathEscape(file.Name)),
		})
	}
	return items
}
