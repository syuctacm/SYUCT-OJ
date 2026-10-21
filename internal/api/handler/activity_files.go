package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/filemeta"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/user"
)

const (
	maxActivityFiles     = 100
	maxActivityFileSize  = 256 << 20
	maxActivityMultipart = 32 << 20

	fileTypePublic  = "public"
	fileTypePrivate = "private"
)

var errPrivateFilesUnsupported = errors.New("private files are not supported")

type activityFileRepository interface {
	Files(context.Context, string) ([]filemeta.File, error)
	FileOwner(context.Context, string) (int64, error)
	UpsertFile(context.Context, string, filemeta.File) error
	DeleteFiles(context.Context, string, []string) error
}

// contestPrivateFileRepository is implemented by the contest repository only.
// Homework documents share that repository, but only contests expose materials.
type contestPrivateFileRepository interface {
	PrivateFiles(context.Context, string) ([]filemeta.File, error)
	UpsertPrivateFile(context.Context, string, filemeta.File) error
	DeletePrivateFiles(context.Context, string, []string) error
	CanReadPrivateFiles(context.Context, string, int64) (bool, error)
}

type ActivityFileHandler struct {
	repo     activityFileRepository
	store    *judge.FileStore
	sessions *auth.SessionManager
	users    *user.Repository
	kind     string
	resource string
}

type activityFile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag,omitempty"`
	LastModified time.Time `json:"lastModified"`
	DownloadURL  string    `json:"downloadUrl"`
}

func NewActivityFileHandler(repo activityFileRepository, store *judge.FileStore, sessions *auth.SessionManager, users *user.Repository, kind, resource string) *ActivityFileHandler {
	return &ActivityFileHandler{repo: repo, store: store, sessions: sessions, users: users, kind: kind, resource: resource}
}

func (h *ActivityFileHandler) RegisterRoutes(router *gin.RouterGroup) {
	base := "/api/v1/" + h.resource + "/:tid/files"
	router.GET(base, h.list)
	router.POST(base, h.upload)
	router.DELETE(base, h.delete)
	router.GET(base+"/:name", h.download)
}

// fileType resolves the `type` query parameter. Only contests have a private
// file area; every other resource always uses the public list.
func (h *ActivityFileHandler) fileType(c *gin.Context) (string, bool) {
	if h.kind != "contest" {
		return fileTypePublic, true
	}
	switch value := c.DefaultQuery("type", fileTypePublic); value {
	case fileTypePublic, fileTypePrivate:
		return value, true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file type"})
		return "", false
	}
}

func (h *ActivityFileHandler) list(c *gin.Context) {
	fileType, ok := h.fileType(c)
	if !ok {
		return
	}
	if fileType == fileTypePrivate {
		if _, ok := h.requireEditor(c); !ok {
			return
		}
	}
	files, err := h.files(c.Request.Context(), fileType, c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "files not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": h.views(c.Param("tid"), fileType, files), "total": len(files),
	}})
}

func (h *ActivityFileHandler) upload(c *gin.Context) {
	fileType, ok := h.fileType(c)
	if !ok {
		return
	}
	if _, ok := h.requireEditor(c); !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file storage is unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxActivityFileSize)
	if err := c.Request.ParseMultipartForm(maxActivityMultipart); err != nil {
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
	existing, err := h.files(c.Request.Context(), fileType, c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "files not found"})
		return
	}
	if len(existing)+len(headers) > maxActivityFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files (max %d)", maxActivityFiles)})
		return
	}
	var total int64
	for _, item := range existing {
		total += item.Size
	}
	for _, header := range headers {
		total += header.Size
	}
	if total > maxActivityFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "files are too large"})
		return
	}
	for _, header := range headers {
		name, err := sanitizeAssetName(path.Base(strings.ReplaceAll(header.Filename, "\\", "/")))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		source, err := header.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to open uploaded file"})
			return
		}
		stored, err := h.put(fileType, c.Param("tid"), name, source)
		_ = source.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save uploaded file"})
			return
		}
		file := filemeta.File{ID: name, Name: name, Size: stored.Size, ETag: stored.ETag, LastModified: stored.LastModified}
		if err := h.upsert(c.Request.Context(), fileType, c.Param("tid"), file); err != nil {
			_ = h.deleteStored(fileType, c.Param("tid"), name)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update file metadata"})
			return
		}
	}
	files, err := h.files(c.Request.Context(), fileType, c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reload files"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": h.views(c.Param("tid"), fileType, files), "total": len(files),
	}})
}

func (h *ActivityFileHandler) delete(c *gin.Context) {
	fileType, ok := h.fileType(c)
	if !ok {
		return
	}
	if _, ok := h.requireEditor(c); !ok {
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
	if err := h.remove(c.Request.Context(), fileType, c.Param("tid"), names); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update file metadata"})
		return
	}
	for _, name := range names {
		_ = h.deleteStored(fileType, c.Param("tid"), name)
	}
	c.Status(http.StatusNoContent)
}

func (h *ActivityFileHandler) download(c *gin.Context) {
	fileType, ok := h.fileType(c)
	if !ok {
		return
	}
	if fileType == fileTypePrivate && !h.authorizePrivateDownload(c) {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file storage is unavailable"})
		return
	}
	name, err := sanitizeAssetName(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	reader, err := h.open(fileType, c.Param("tid"), name)
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
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

// authorizePrivateDownload mirrors the upstream rule: owners and system
// administrators always pass, participants only after the contest started.
func (h *ActivityFileHandler) authorizePrivateDownload(c *gin.Context) bool {
	u, ok := h.authenticatedUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return false
	}
	if u.Priv&1 != 0 {
		return true
	}
	repo, ok := h.repo.(contestPrivateFileRepository)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "files not found"})
		return false
	}
	allowed, err := repo.CanReadPrivateFiles(c.Request.Context(), c.Param("tid"), u.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "files not found"})
		return false
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

func (h *ActivityFileHandler) files(ctx context.Context, fileType, tid string) ([]filemeta.File, error) {
	if fileType != fileTypePrivate {
		return h.repo.Files(ctx, tid)
	}
	repo, ok := h.repo.(contestPrivateFileRepository)
	if !ok {
		return nil, errPrivateFilesUnsupported
	}
	return repo.PrivateFiles(ctx, tid)
}

func (h *ActivityFileHandler) upsert(ctx context.Context, fileType, tid string, file filemeta.File) error {
	if fileType != fileTypePrivate {
		return h.repo.UpsertFile(ctx, tid, file)
	}
	repo, ok := h.repo.(contestPrivateFileRepository)
	if !ok {
		return errPrivateFilesUnsupported
	}
	return repo.UpsertPrivateFile(ctx, tid, file)
}

func (h *ActivityFileHandler) remove(ctx context.Context, fileType, tid string, names []string) error {
	if fileType != fileTypePrivate {
		return h.repo.DeleteFiles(ctx, tid, names)
	}
	repo, ok := h.repo.(contestPrivateFileRepository)
	if !ok {
		return errPrivateFilesUnsupported
	}
	return repo.DeletePrivateFiles(ctx, tid, names)
}

func (h *ActivityFileHandler) views(tid, fileType string, files []filemeta.File) []activityFile {
	items := make([]activityFile, 0, len(files))
	for _, file := range files {
		downloadURL := fmt.Sprintf("/api/v1/%s/%s/files/%s", h.resource, tid, url.PathEscape(file.Name))
		if fileType == fileTypePrivate {
			downloadURL += "?type=" + fileTypePrivate
		}
		items = append(items, activityFile{
			ID: file.ID, Name: file.Name, Size: file.Size, ETag: file.ETag, LastModified: file.LastModified,
			DownloadURL: downloadURL,
		})
	}
	return items
}

func (h *ActivityFileHandler) put(fileType, tid, name string, reader io.Reader) (judge.StoredFile, error) {
	if h.kind == "training" {
		return h.store.PutTrainingFile("system", tid, name, reader)
	}
	if fileType == fileTypePrivate {
		return h.store.PutContestPrivateFile("system", tid, name, reader)
	}
	return h.store.PutContestFile("system", tid, name, reader)
}

func (h *ActivityFileHandler) open(fileType, tid, name string) (io.ReadCloser, error) {
	if h.kind == "training" {
		return h.store.OpenTrainingFile("system", tid, name)
	}
	if fileType == fileTypePrivate {
		return h.store.OpenContestPrivateFile("system", tid, name)
	}
	return h.store.OpenContestFile("system", tid, name)
}

func (h *ActivityFileHandler) deleteStored(fileType, tid, name string) error {
	if h.kind == "training" {
		return h.store.DeleteTrainingFile("system", tid, name)
	}
	if fileType == fileTypePrivate {
		return h.store.DeleteContestPrivateFile("system", tid, name)
	}
	return h.store.DeleteContestFile("system", tid, name)
}

func (h *ActivityFileHandler) requireEditor(c *gin.Context) (int64, bool) {
	u, ok := h.authenticatedUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	owner, err := h.repo.FileOwner(c.Request.Context(), c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "owner not found"})
		return 0, false
	}
	if owner != u.ID && u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return 0, false
	}
	return u.ID, true
}

func (h *ActivityFileHandler) authenticatedUser(c *gin.Context) (*user.User, bool) {
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
