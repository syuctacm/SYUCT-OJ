package handler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

const maxFPSMultipart = problem.MaxFPSImportSize + (8 << 20)

// importFPS creates problems from an uploaded FPS problemset, ported from the
// upstream fps-importer addon. The body is multipart/form-data with a `file`
// field holding either one XML document or a zip archive of XML documents.
func (h *ProblemHandler) importFPS(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "problem file storage is unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFPSMultipart)
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if header.Size > problem.MaxFPSImportSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "problemset is too large"})
		return
	}
	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read uploaded file"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, problem.MaxFPSImportSize))
	_ = file.Close()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read uploaded file"})
		return
	}
	items, err := problem.ParseFPS(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created := make([]gin.H, 0, len(items))
	failures := make([]gin.H, 0)
	for _, item := range items {
		pdoc, err := h.importFPSProblem(c, u.ID, item)
		if err != nil {
			failures = append(failures, gin.H{"title": item.Title, "error": err.Error()})
			continue
		}
		created = append(created, gin.H{"pid": pdoc.PID, "docId": pdoc.DocID, "title": pdoc.Title})
	}
	if len(created) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no problem could be imported", "failures": failures})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": created, "failures": failures,
	}})
}

func (h *ProblemHandler) importFPSProblem(c *gin.Context, owner int64, item problem.FPSProblem) (*problem.Problem, error) {
	ctx := c.Request.Context()
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("problemset item has no title")
	}
	pdoc, err := h.repo.Create(ctx, owner, problem.EditInput{
		Title:   title,
		Content: item.Content(),
		Tags:    item.Tags(),
		Config:  item.Config(),
		Html:    !item.Markdown,
	})
	if err != nil {
		return nil, err
	}
	files := item.Testdata
	if config := item.Config(); config != "" {
		files = append([]problem.FPSFile{{Name: "config.yaml", Content: config}}, files...)
	}
	for _, file := range files {
		if err := h.importStoreFile(c, pdoc.DocID, problemFileKindData, file.Name, strings.NewReader(file.Content)); err != nil {
			return nil, err
		}
	}
	content := pdoc.Content
	for _, image := range item.Images {
		if image.Src == "" || image.Base64 == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(image.Base64)
		if err != nil {
			continue
		}
		name := problem.ImageName(image.Src)
		if err := h.importStoreFile(c, pdoc.DocID, problemFileKindAttach, name, bytes.NewReader(raw)); err != nil {
			return nil, err
		}
		content = strings.ReplaceAll(content, image.Src, "file://"+name)
	}
	if content != pdoc.Content {
		if err := h.repo.UpdateContent(ctx, pdoc.DocID, content); err != nil {
			return nil, err
		}
	}
	for _, solution := range item.Solutions {
		if strings.TrimSpace(solution.Content) == "" {
			continue
		}
		body := fmt.Sprintf("**%s** :  \n```\n%s\n```\n", solution.Language, solution.Content)
		if _, err := h.repo.CreateSolution(ctx, pdoc.DocID, owner, body); err != nil {
			return nil, err
		}
	}
	return h.repo.GetForEditor(ctx, strconv.FormatInt(pdoc.DocID, 10))
}

// importStoreFile writes one file and registers its metadata on the problem.
func (h *ProblemHandler) importStoreFile(c *gin.Context, docID int64, kind, name string, source io.Reader) error {
	ctx := c.Request.Context()
	id := strconv.FormatInt(docID, 10)
	var stored judge.StoredFile
	var err error
	if kind == problemFileKindData {
		stored, err = h.store.PutProblemFile(problem.DomainSystem, id, name, source)
	} else {
		stored, err = h.store.PutAdditionalFile(problem.DomainSystem, id, name, source)
	}
	if err != nil {
		return fmt.Errorf("store %s: %w", name, err)
	}
	return h.repo.UpsertFile(ctx, docID, kind, problem.ProblemFile{
		ID:           problemFileID(docID, kind, name),
		Name:         name,
		Size:         stored.Size,
		ETag:         stored.ETag,
		LastModified: stored.LastModified,
	})
}
