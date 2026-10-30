package handler

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gopkg.in/yaml.v3"

	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/user"
)

const (
	maxProblemFiles       = 100
	maxProblemFilesSize   = 256 << 20
	maxProblemMultipart   = 32 << 20
	problemFileKindData   = "testdata"
	problemFileKindAttach = "additional"
)

type ProblemHandler struct {
	repo        *problem.Repository
	sessions    *auth.SessionManager
	users       *user.Repository
	store       *judge.FileStore
	submissions *submission.Repository
}

func NewProblemHandler(repo *problem.Repository, sessions *auth.SessionManager, users *user.Repository, store *judge.FileStore, submissions *submission.Repository) *ProblemHandler {
	return &ProblemHandler{repo: repo, sessions: sessions, users: users, store: store, submissions: submissions}
}

func (h *ProblemHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/problems", h.list)
	router.GET("/api/v1/problem-tags", h.tags)
	router.GET("/api/v1/problem-tags/:tag/ranking", h.tagRanking)
	router.GET("/api/v1/problem-tags/:tag/ranking/export", h.tagRankingExport)
	router.POST("/api/v1/problems", h.create)
	router.POST("/api/v1/problems/import/fps", h.importFPS)
	router.POST("/api/v1/problems/import/hydro", h.importHydro)
	router.GET("/api/v1/problems/random", h.random)
	router.GET("/api/v1/problems/:pid/edit", h.editData)
	router.PATCH("/api/v1/problems/:pid", h.update)
	router.DELETE("/api/v1/problems/:pid", h.delete)
	router.GET("/api/v1/problems/:pid/statistics", h.statistics)
	router.POST("/api/v1/problems/:pid/rejudge", h.rejudge)
	router.GET("/api/v1/problems/:pid/assets", h.listAssets)
	router.POST("/api/v1/problems/:pid/assets", h.uploadAssets)
	router.DELETE("/api/v1/problems/:pid/assets", h.deleteAssets)
	router.POST("/api/v1/problems/:pid/assets/rename", h.renameAssets)
	router.POST("/api/v1/problems/:pid/assets/archive", h.archiveAssets)
	router.GET("/api/v1/problems/:pid/assets/download", h.downloadAsset)
	router.GET("/api/v1/problems/:pid/assets/download-all", h.downloadAllAssets)
	router.GET("/api/v1/problems/:pid", h.get)
	router.GET("/api/v1/starred-problems", h.starred)
	router.POST("/api/v1/problems/:pid/star", h.star)
}

type problemEditRequest struct {
	PID        string   `json:"pid"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	Difficulty int      `json:"difficulty"`
	Hidden     bool     `json:"hidden"`
	Config     string   `json:"config"`
}

type problemEditorView struct {
	Problem *problem.Problem `json:"problem"`
	Config  string           `json:"config"`
}

func (h *ProblemHandler) create(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	input, ok := bindProblemEdit(c)
	if !ok {
		return
	}
	item, err := h.repo.Create(c.Request.Context(), u.ID, input)
	if err != nil {
		writeProblemEditError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": editorView(item)})
}

func (h *ProblemHandler) editData(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": editorView(pdoc)})
}

func (h *ProblemHandler) update(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	input, ok := bindProblemEdit(c)
	if !ok {
		return
	}
	item, err := h.repo.Update(c.Request.Context(), pdoc.DocID, input)
	if err != nil {
		writeProblemEditError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": editorView(item)})
}

func bindProblemEdit(c *gin.Context) (problem.EditInput, bool) {
	var request problemEditRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return problem.EditInput{}, false
	}
	request.PID = strings.TrimSpace(request.PID)
	request.Title = strings.TrimSpace(request.Title)
	if request.PID == "" || request.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pid and title are required"})
		return problem.EditInput{}, false
	}
	if request.Difficulty < 0 || request.Difficulty > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "difficulty must be between 0 and 10"})
		return problem.EditInput{}, false
	}
	config, err := normalizeProblemConfig(request.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return problem.EditInput{}, false
	}
	tags := make([]string, 0, len(request.Tags))
	seen := map[string]bool{}
	for _, value := range request.Tags {
		tag := strings.TrimSpace(value)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return problem.EditInput{
		PID: request.PID, Title: request.Title, Content: request.Content,
		Tags: tags, Difficulty: request.Difficulty, Hidden: request.Hidden, Config: config,
	}, true
}

func normalizeProblemConfig(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	var value any
	if err := yaml.Unmarshal([]byte(raw), &value); err != nil {
		return "", fmt.Errorf("invalid judge config YAML: %w", err)
	}
	config, err := configMap(value)
	if err != nil {
		return "", err
	}
	encoded, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("normalize judge config: %w", err)
	}
	return string(encoded), nil
}

func configMap(value any) (map[string]any, error) {
	switch current := value.(type) {
	case map[string]any:
		if current == nil {
			return nil, errors.New("judge config must be a YAML object")
		}
		return current, nil
	case []any:
		result := make(map[string]any, len(current))
		for _, item := range current {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("judge config list entries must contain key and value")
			}
			key, ok := entry["key"].(string)
			if !ok || strings.TrimSpace(key) == "" {
				return nil, errors.New("judge config list entries must contain a non-empty key")
			}
			result[key] = entry["value"]
		}
		return result, nil
	default:
		return nil, errors.New("judge config must be a YAML object")
	}
}

func editorView(item *problem.Problem) problemEditorView {
	return problemEditorView{Problem: item, Config: problemConfigString(item)}
}

func problemConfigString(item *problem.Problem) string {
	if item.Config == nil {
		return ""
	}
	var value any
	switch current := item.Config.(type) {
	case string:
		if strings.TrimSpace(current) == "" {
			return ""
		}
		if err := yaml.Unmarshal([]byte(current), &value); err != nil {
			return current
		}
	case bson.M:
		value = map[string]any(current)
	case bson.D:
		converted := make(map[string]any, len(current))
		for _, entry := range current {
			converted[entry.Key] = entry.Value
		}
		value = converted
	default:
		value = current
	}
	config, err := configMap(value)
	if err != nil {
		return ""
	}
	encoded, err := yaml.Marshal(config)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func writeProblemEditError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, problem.ErrDuplicatePID):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, problem.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// random answers the Lucky button: one random visible problem, filtered by the
// category: tokens of the current search query.
func (h *ProblemHandler) random(c *gin.Context) {
	tags := randomTags(c.Query("q"))
	pid, err := h.repo.Random(c.Request.Context(), tags)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no problem found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"pid": pid}})
}

func randomTags(query string) []string {
	tags := make([]string, 0)
	for _, token := range strings.Fields(query) {
		if !strings.HasPrefix(token, "category:") {
			continue
		}
		for _, tag := range strings.Split(strings.TrimPrefix(token, "category:"), ",") {
			tag = strings.Trim(strings.TrimSpace(tag), `"`)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func (h *ProblemHandler) list(c *gin.Context) {
	var uid int64
	if h.sessions != nil {
		if session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c)); ok && session.Kind == "user" {
			uid = session.UserID
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	var difficulty *int
	if raw := c.Query("difficulty"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			difficulty = &value
		}
	}
	includeHidden := false
	if uid > 0 && h.users != nil {
		if account, err := h.users.FindByID(c.Request.Context(), uid); err == nil && account.Priv&1 != 0 {
			includeHidden = true
		}
	}
	result, err := h.repo.List(c.Request.Context(), problem.ListOptions{
		Page:          page,
		Limit:         limit,
		Query:         c.Query("q"),
		Tag:           c.Query("tag"),
		Difficulty:    difficulty,
		Sort:          c.Query("sort"),
		UID:           uid,
		IncludeHidden: includeHidden,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to list problems"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *ProblemHandler) tags(c *gin.Context) {
	items, err := h.repo.TagStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load tag statistics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"items": items}})
}

func (h *ProblemHandler) tagRanking(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.repo.TagRanking(c.Request.Context(), c.Param("tag"), page, limit, c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load tag ranking"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *ProblemHandler) tagRankingExport(c *gin.Context) {
	tag := c.Param("tag")
	items, err := h.repo.TagRankingExport(c.Request.Context(), tag, c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to export tag ranking"})
		return
	}
	var output bytes.Buffer
	output.WriteString("\uFEFF")
	writer := csv.NewWriter(&output)
	_ = writer.Write([]string{"排名", "UID", "账号名", "过题数", "提交数", "通过率", "算法标签"})
	for _, item := range items {
		rate := 0.0
		if item.Submissions > 0 {
			rate = float64(item.Solved) / float64(item.Submissions) * 100
		}
		_ = writer.Write([]string{
			strconv.Itoa(item.Rank),
			strconv.FormatInt(item.UID, 10),
			item.Username,
			strconv.FormatInt(item.Solved, 10),
			strconv.FormatInt(item.Submissions, 10),
			fmt.Sprintf("%.1f%%", rate),
			tag,
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to encode tag ranking"})
		return
	}
	filename := fmt.Sprintf("%s标签过题榜单_%s.csv", tag, time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(output.Bytes())
}

func (h *ProblemHandler) get(c *gin.Context) {
	pdoc, err := h.repo.Get(c.Request.Context(), c.Param("pid"))
	if err != nil {
		// Hidden problems stay visible to their owner and to administrators,
		// mirroring Hydro's PERM_VIEW_PROBLEM_HIDDEN check.
		if uid, ok := h.currentUser(c); ok {
			if hiddenDoc, hiddenErr := h.repo.GetForEditor(c.Request.Context(), c.Param("pid")); hiddenErr == nil {
				account, accountErr := h.users.FindByID(c.Request.Context(), uid)
				if accountErr == nil && (hiddenDoc.Owner == uid || account.Priv&1 != 0) {
					pdoc, err = hiddenDoc, nil
				}
			}
		}
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}
	if uid, ok := h.currentUser(c); ok {
		starred, err := h.repo.StarredIDs(c.Request.Context(), uid, []int64{pdoc.DocID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load problem status"})
			return
		}
		pdoc.Starred = starred[pdoc.DocID]
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": pdoc})
}

// starred lists the problems the caller starred, newest first.
func (h *ProblemHandler) starred(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.repo.Starred(c.Request.Context(), uid, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load starred problems"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": items, "total": len(items),
	}})
}

// star toggles the caller's star flag on one problem.
func (h *ProblemHandler) star(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	pdoc, err := h.repo.Get(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}
	var request struct {
		Star bool `json:"star"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.repo.Star(c.Request.Context(), pdoc.DocID, uid, request.Star); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update problem status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"star": request.Star}})
}

func (h *ProblemHandler) currentUser(c *gin.Context) (int64, bool) {
	if h.sessions == nil {
		return 0, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		return 0, false
	}
	return session.UserID, true
}

type problemAsset struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag,omitempty"`
	LastModified time.Time `json:"lastModified"`
	DownloadURL  string    `json:"downloadUrl"`
}

type problemAssetList struct {
	Problem    *problem.Problem `json:"problem"`
	Testdata   []problemAsset   `json:"testdata"`
	Additional []problemAsset   `json:"additional"`
}

func (h *ProblemHandler) listAssets(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": assetList(c.Param("pid"), pdoc),
	})
}

func (h *ProblemHandler) uploadAssets(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "problem file storage is unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProblemFilesSize)
	if err := c.Request.ParseMultipartForm(maxProblemMultipart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart upload"})
		return
	}
	kind := c.PostForm("type")
	if kind == "" {
		kind = problemFileKindData
	}
	if !validProblemFileKind(kind) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file type"})
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
	existing := assetsForKind(pdoc, kind)
	if len(existing)+len(headers) > maxProblemFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files (max %d)", maxProblemFiles)})
		return
	}
	var totalSize int64
	for _, existingFile := range existing {
		totalSize += existingFile.Size
	}
	for _, header := range headers {
		totalSize += header.Size
	}
	if totalSize > maxProblemFilesSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "problem files are too large"})
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
		var stored judge.StoredFile
		if kind == problemFileKindData {
			stored, err = h.store.PutProblemFile(problem.DomainSystem, strconv.FormatInt(pdoc.DocID, 10), name, source)
		} else {
			stored, err = h.store.PutAdditionalFile(problem.DomainSystem, strconv.FormatInt(pdoc.DocID, 10), name, source)
		}
		_ = source.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to save uploaded file"})
			return
		}
		file := problem.ProblemFile{
			ID:           problemFileID(pdoc.DocID, kind, name),
			Name:         name,
			Size:         stored.Size,
			ETag:         stored.ETag,
			LastModified: stored.LastModified,
		}
		if err := h.repo.UpsertFile(c.Request.Context(), pdoc.DocID, kind, file); err != nil {
			h.deleteStoredFile(kind, pdoc.DocID, name)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update problem file metadata"})
			return
		}
		if kind == problemFileKindData && isConfigFile(name) {
			if err := h.updateProblemConfig(c, pdoc.DocID, name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update problem config"})
				return
			}
		}
	}
	updated, err := h.repo.GetForEditor(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reload problem files"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": assetList(c.Param("pid"), updated)})
}

func (h *ProblemHandler) deleteAssets(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	var request struct {
		Type  string   `json:"type"`
		Names []string `json:"names"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Names) == 0 || !validProblemFileKind(request.Type) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type and names are required"})
		return
	}
	if len(request.Names) > maxProblemFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files (max %d)", maxProblemFiles)})
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
	if err := h.repo.DeleteFiles(c.Request.Context(), pdoc.DocID, request.Type, names); err != nil && !errors.Is(err, problem.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update problem file metadata"})
		return
	}
	for _, name := range names {
		h.deleteStoredFile(request.Type, pdoc.DocID, name)
		if request.Type == problemFileKindData && isConfigFile(name) {
			_ = h.repo.UpdateConfig(c.Request.Context(), pdoc.DocID, "")
		}
	}
	updated, err := h.repo.GetForEditor(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reload problem files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": assetList(c.Param("pid"), updated)})
}

func (h *ProblemHandler) archiveAssets(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "problem file storage is unavailable"})
		return
	}
	var request struct {
		Type  string   `json:"type"`
		Names []string `json:"names"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || !validProblemFileKind(request.Type) || len(request.Names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type and names are required"})
		return
	}
	if len(request.Names) > maxProblemFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files (max %d)", maxProblemFiles)})
		return
	}
	existing := assetsForKind(pdoc, request.Type)
	byName := make(map[string]problem.ProblemFile, len(existing))
	for _, file := range existing {
		byName[file.Name] = file
	}
	names := make([]string, 0, len(request.Names))
	seen := map[string]bool{}
	for _, raw := range request.Names {
		name, err := sanitizeAssetName(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if seen[name] {
			continue
		}
		file, found := byName[name]
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("file not found: %s", name)})
			return
		}
		reader, err := h.openAsset(request.Type, pdoc.DocID, file.Name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("file not found: %s", name)})
			return
		}
		_ = reader.Close()
		seen[name] = true
		names = append(names, name)
	}
	filename := fmt.Sprintf("%d %s %s.zip", pdoc.DocID, pdoc.Title, request.Type)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Content-Type", "application/zip")
	writer := zip.NewWriter(c.Writer)
	defer writer.Close()
	for _, name := range names {
		reader, err := h.openAsset(request.Type, pdoc.DocID, name)
		if err != nil {
			return
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if file, ok := byName[name]; ok && !file.LastModified.IsZero() {
			header.SetModTime(file.LastModified)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = reader.Close()
			return
		}
		_, copyErr := io.Copy(entry, reader)
		_ = reader.Close()
		if copyErr != nil {
			return
		}
	}
}

func (h *ProblemHandler) renameAssets(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "problem file storage is unavailable"})
		return
	}
	var request struct {
		Type     string   `json:"type"`
		Names    []string `json:"names"`
		NewNames []string `json:"newNames"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || !validProblemFileKind(request.Type) || len(request.Names) == 0 || len(request.Names) != len(request.NewNames) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type, names and newNames are required"})
		return
	}
	if len(request.Names) > maxProblemFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("too many files (max %d)", maxProblemFiles)})
		return
	}
	existing := assetsForKind(pdoc, request.Type)
	existingNames := make(map[string]bool, len(existing))
	for _, file := range existing {
		existingNames[file.Name] = true
	}
	requestedOldNames := make(map[string]bool, len(request.Names))
	for _, raw := range request.Names {
		name, err := sanitizeAssetName(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		requestedOldNames[name] = true
	}
	renames := make([]struct{ old, new string }, 0, len(request.Names))
	oldNames := make(map[string]bool, len(request.Names))
	newNames := make(map[string]bool, len(request.Names))
	for index, raw := range request.Names {
		oldName, err := sanitizeAssetName(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		newName, err := sanitizeAssetName(request.NewNames[index])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if oldName == newName {
			c.JSON(http.StatusBadRequest, gin.H{"error": "new filename must differ from the original"})
			return
		}
		if !existingNames[oldName] {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("file not found: %s", oldName)})
			return
		}
		if oldNames[oldName] || newNames[newName] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duplicate filename in rename request"})
			return
		}
		if existingNames[newName] && !requestedOldNames[newName] {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("file already exists: %s", newName)})
			return
		}
		oldNames[oldName] = true
		newNames[newName] = true
		renames = append(renames, struct{ old, new string }{oldName, newName})
	}

	created := make([]string, 0, len(renames))
	rollback := func() {
		_ = h.repo.DeleteFiles(c.Request.Context(), pdoc.DocID, request.Type, created)
		for _, name := range created {
			h.deleteStoredFile(request.Type, pdoc.DocID, name)
		}
	}
	for _, item := range renames {
		reader, err := h.openAsset(request.Type, pdoc.DocID, item.old)
		if err != nil {
			rollback()
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("file not found: %s", item.old)})
			return
		}
		stored, err := h.putAsset(request.Type, pdoc.DocID, item.new, reader)
		_ = reader.Close()
		if err != nil {
			rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to copy problem file"})
			return
		}
		created = append(created, item.new)
		file := problem.ProblemFile{
			ID: problemFileID(pdoc.DocID, request.Type, item.new), Name: item.new,
			Size: stored.Size, ETag: stored.ETag, LastModified: stored.LastModified,
		}
		if err := h.repo.UpsertFile(c.Request.Context(), pdoc.DocID, request.Type, file); err != nil {
			rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update problem file metadata"})
			return
		}
	}
	oldList := make([]string, 0, len(renames))
	for _, item := range renames {
		oldList = append(oldList, item.old)
	}
	if err := h.repo.DeleteFiles(c.Request.Context(), pdoc.DocID, request.Type, oldList); err != nil && !errors.Is(err, problem.ErrNotFound) {
		rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update problem file metadata"})
		return
	}
	hadConfig := false
	hasConfig := false
	for _, item := range renames {
		h.deleteStoredFile(request.Type, pdoc.DocID, item.old)
		if request.Type == problemFileKindData && isConfigFile(item.old) {
			hadConfig = true
		}
		if request.Type == problemFileKindData && isConfigFile(item.new) {
			hasConfig = true
		}
	}
	if hadConfig && !hasConfig {
		_ = h.repo.UpdateConfig(c.Request.Context(), pdoc.DocID, "")
	}
	if hasConfig {
		for _, item := range renames {
			if isConfigFile(item.new) {
				if err := h.updateProblemConfig(c, pdoc.DocID, item.new); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reload problem config"})
					return
				}
			}
		}
	}
	updated, err := h.repo.GetForEditor(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reload problem files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": assetList(c.Param("pid"), updated)})
}

func (h *ProblemHandler) openAsset(kind string, docID int64, name string) (io.ReadCloser, error) {
	pid := strconv.FormatInt(docID, 10)
	if kind == problemFileKindData {
		return h.store.OpenProblemFile(problem.DomainSystem, pid, name)
	}
	return h.store.OpenAdditionalFile(problem.DomainSystem, pid, name)
}

func (h *ProblemHandler) putAsset(kind string, docID int64, name string, reader io.Reader) (judge.StoredFile, error) {
	pid := strconv.FormatInt(docID, 10)
	if kind == problemFileKindData {
		return h.store.PutProblemFile(problem.DomainSystem, pid, name, reader)
	}
	return h.store.PutAdditionalFile(problem.DomainSystem, pid, name, reader)
}

func (h *ProblemHandler) downloadAsset(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "problem file storage is unavailable"})
		return
	}
	kind := c.Query("type")
	if !validProblemFileKind(kind) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file type"})
		return
	}
	name, err := sanitizeAssetName(c.Query("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var reader io.ReadCloser
	if kind == problemFileKindData {
		reader, err = h.store.OpenProblemFile(problem.DomainSystem, strconv.FormatInt(pdoc.DocID, 10), name)
	} else {
		reader, err = h.store.OpenAdditionalFile(problem.DomainSystem, strconv.FormatInt(pdoc.DocID, 10), name)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	defer reader.Close()
	contentDisposition := mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(name)})
	c.Header("Content-Disposition", contentDisposition)
	c.Header("Content-Type", "application/octet-stream")
	_, _ = io.Copy(c.Writer, reader)
}

func (h *ProblemHandler) downloadAllAssets(c *gin.Context) {
	pdoc, ok := h.requireEditor(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "problem file storage is unavailable"})
		return
	}
	configText := problemConfigString(pdoc)
	hasConfigFile := false
	for _, file := range pdoc.Data {
		if isConfigFile(file.Name) {
			hasConfigFile = true
			break
		}
	}
	if len(pdoc.Data) == 0 && strings.TrimSpace(configText) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "testdata is empty"})
		return
	}

	pid := strconv.FormatInt(pdoc.DocID, 10)
	// Validate every object before writing response headers so a missing
	// file produces a proper JSON error instead of a truncated zip.
	for _, file := range pdoc.Data {
		name, err := sanitizeAssetName(file.Name)
		if err != nil || name != file.Name {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid stored filename"})
			return
		}
		reader, err := h.store.OpenProblemFile(problem.DomainSystem, pid, name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("file not found: %s", file.Name)})
			return
		}
		_ = reader.Close()
	}

	filename := fmt.Sprintf("%d %s.zip", pdoc.DocID, pdoc.PID)
	if pdoc.Title != "" {
		filename = fmt.Sprintf("%d %s.zip", pdoc.DocID, pdoc.Title)
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Content-Type", "application/zip")
	writer := zip.NewWriter(c.Writer)
	defer writer.Close()
	for _, file := range pdoc.Data {
		name, err := sanitizeAssetName(file.Name)
		if err != nil || name != file.Name {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid stored filename"})
			return
		}
		reader, err := h.store.OpenProblemFile(problem.DomainSystem, pid, name)
		if err != nil {
			return
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if !file.LastModified.IsZero() {
			header.SetModTime(file.LastModified)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = reader.Close()
			return
		}
		_, copyErr := io.Copy(entry, reader)
		_ = reader.Close()
		if copyErr != nil {
			return
		}
	}
	if !hasConfigFile && strings.TrimSpace(configText) != "" {
		entry, err := writer.Create("config.yaml")
		if err != nil {
			return
		}
		if _, err := io.WriteString(entry, configText); err != nil {
			return
		}
	}
}

func (h *ProblemHandler) requireAdmin(c *gin.Context) (*user.User, bool) {
	if h.sessions == nil || h.users == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	u, err := h.users.FindByID(c.Request.Context(), session.UserID)
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

func (h *ProblemHandler) requireEditor(c *gin.Context) (*problem.Problem, bool) {
	if h.sessions == nil || h.users == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	session, ok := h.sessions.GetContext(c.Request.Context(), auth.Token(c))
	if !ok || session.Kind != "user" || session.UserID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	u, err := h.users.FindByID(c.Request.Context(), session.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	pdoc, err := h.repo.GetForEditor(c.Request.Context(), c.Param("pid"))
	if err != nil {
		if errors.Is(err, problem.ErrNotFound) || errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load problem"})
		}
		return nil, false
	}
	if pdoc.Owner != session.UserID && u.Priv&1 == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return pdoc, true
}

func (h *ProblemHandler) updateProblemConfig(c *gin.Context, docID int64, name string) error {
	reader, err := h.store.OpenProblemFile(problem.DomainSystem, strconv.FormatInt(docID, 10), name)
	if err != nil {
		return err
	}
	defer reader.Close()
	value, err := io.ReadAll(io.LimitReader(reader, 2<<20))
	if err != nil {
		return err
	}
	return h.repo.UpdateConfig(c.Request.Context(), docID, string(value))
}

func (h *ProblemHandler) deleteStoredFile(kind string, docID int64, name string) {
	if h.store == nil {
		return
	}
	pid := strconv.FormatInt(docID, 10)
	if kind == problemFileKindData {
		_ = h.store.DeleteProblemFile(problem.DomainSystem, pid, name)
	} else {
		_ = h.store.DeleteAdditionalFile(problem.DomainSystem, pid, name)
	}
}

func assetList(pid string, pdoc *problem.Problem) problemAssetList {
	return problemAssetList{
		Problem:    pdoc,
		Testdata:   assetViews(pid, problemFileKindData, pdoc.Data),
		Additional: assetViews(pid, problemFileKindAttach, pdoc.AdditionalFiles),
	}
}

func assetViews(pid, kind string, files []problem.ProblemFile) []problemAsset {
	result := make([]problemAsset, 0, len(files))
	for _, file := range files {
		result = append(result, problemAsset{
			ID:           file.ID,
			Name:         file.Name,
			Size:         file.Size,
			ETag:         file.ETag,
			LastModified: file.LastModified,
			DownloadURL:  "/api/v1/problems/" + url.PathEscape(pid) + "/assets/download?type=" + url.QueryEscape(kind) + "&name=" + url.QueryEscape(file.Name),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func assetsForKind(pdoc *problem.Problem, kind string) []problem.ProblemFile {
	if kind == problemFileKindData {
		return pdoc.Data
	}
	return pdoc.AdditionalFiles
}

func validProblemFileKind(kind string) bool {
	return kind == problemFileKindData || kind == problemFileKindAttach
}

func sanitizeAssetName(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "", errors.New("filename is required")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || strings.ContainsRune(cleaned, '\x00') {
		return "", fmt.Errorf("invalid filename %q", value)
	}
	return cleaned, nil
}

func isConfigFile(name string) bool {
	switch strings.ToLower(path.Base(name)) {
	case "config.yaml", "config.yml":
		return true
	default:
		return false
	}
}

func problemFileID(docID int64, kind, name string) string {
	folder := "testdata"
	if kind == problemFileKindAttach {
		folder = "additional_file"
	}
	return fmt.Sprintf("problem/%s/%d/%s/%s", problem.DomainSystem, docID, folder, name)
}
