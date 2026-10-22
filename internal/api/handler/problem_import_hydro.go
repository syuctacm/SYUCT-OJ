package handler

import (
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/problem"
)

const maxHydroMultipart = problem.MaxHydroImportSize + (32 << 20)

var problemIDPrefixPattern = regexp.MustCompile(`^[a-zA-Z]+$`)

// importHydro creates problems from a Hydro / ICPC zip package, ported from
// ProblemImportHydroHandler. Supported fields mirror the upstream form:
// preferredPrefix, hidden and keepUser.
func (h *ProblemHandler) importHydro(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "problem file storage is unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxHydroMultipart)
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if header.Size > problem.MaxHydroImportSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "problemset is too large"})
		return
	}
	preferredPrefix := strings.TrimSpace(c.PostForm("preferredPrefix"))
	if preferredPrefix != "" && !problemIDPrefixPattern.MatchString(preferredPrefix) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "preferredPrefix must contain letters only"})
		return
	}
	hidden := parseFormBool(c.PostForm("hidden"))
	keepUser := parseFormBool(c.PostForm("keepUser"))
	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read uploaded file"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, problem.MaxHydroImportSize))
	_ = file.Close()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read uploaded file"})
		return
	}
	packages, err := problem.ParseHydroPackages(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created := make([]gin.H, 0, len(packages))
	failures := make([]gin.H, 0)
	for _, pkg := range packages {
		pdoc, err := h.importHydroPackage(c, u.ID, pkg, preferredPrefix, hidden, keepUser)
		if err != nil {
			failures = append(failures, gin.H{"title": pkg.Title, "error": err.Error()})
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

func (h *ProblemHandler) importHydroPackage(c *gin.Context, admin int64, pkg problem.HydroPackage, preferredPrefix string, hidden, keepUser bool) (*problem.Problem, error) {
	ctx := c.Request.Context()
	owner := admin
	if keepUser && pkg.Owner > 0 {
		owner = pkg.Owner
	}
	pid := applyPreferredPrefix(pkg.PID, preferredPrefix)
	input := problem.EditInput{
		PID:        pid,
		Title:      pkg.Title,
		Content:    pkg.Content,
		Tags:       pkg.Tags,
		Difficulty: pkg.Difficulty,
		Hidden:     hidden || pkg.Hidden,
		Config:     pkg.Config,
	}
	pdoc, err := h.repo.Create(ctx, owner, input)
	if err == problem.ErrDuplicatePID {
		// Upstream keeps the existing problem and imports under a fresh id.
		input.PID = ""
		pdoc, err = h.repo.Create(ctx, owner, input)
	}
	if err != nil {
		return nil, err
	}
	for _, file := range pkg.Testdata {
		if err := h.importStoreFile(c, pdoc.DocID, problemFileKindData, file.Name, strings.NewReader(file.Content)); err != nil {
			return nil, err
		}
	}
	for _, file := range pkg.Additional {
		if err := h.importStoreFile(c, pdoc.DocID, problemFileKindAttach, file.Name, strings.NewReader(file.Content)); err != nil {
			return nil, err
		}
	}
	for _, solution := range pkg.Solutions {
		if strings.TrimSpace(solution) == "" {
			continue
		}
		if _, err := h.repo.CreateSolution(ctx, pdoc.DocID, owner, solution); err != nil {
			return nil, err
		}
	}
	return h.repo.GetForEditor(ctx, strconv.FormatInt(pdoc.DocID, 10))
}

// applyPreferredPrefix rewrites the leading letters of a pid, like the upstream
// preferredPrefix option.
func applyPreferredPrefix(pid, prefix string) string {
	if prefix == "" || pid == "" {
		return pid
	}
	trimmed := strings.TrimLeft(pid, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	return prefix + trimmed
}

func parseFormBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}
