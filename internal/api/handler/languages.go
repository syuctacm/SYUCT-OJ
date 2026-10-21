package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
)

// LanguageHandler exposes the judge language registry so the frontend no longer
// keeps its own hardcoded list.
type LanguageHandler struct {
	problems *problem.Repository
	contests *contest.Repository
}

func NewLanguageHandler(problems *problem.Repository, contests *contest.Repository) *LanguageHandler {
	return &LanguageHandler{problems: problems, contests: contests}
}

func (h *LanguageHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/languages", h.list)
	router.GET("/api/v1/problems/:pid/langs", h.problemLangs)
}

func (h *LanguageHandler) list(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": judge.LanguageOptions(),
	}})
}

// problemLangs intersects the judge registry with the problem config `langs`
// and, when a contest is given, the contest language restrictions.
func (h *LanguageHandler) problemLangs(c *gin.Context) {
	pdoc, err := h.problems.Get(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}
	constraints := [][]string{pdoc.Langs}
	if tid := strings.TrimSpace(c.Query("contest")); tid != "" && h.contests != nil {
		item, err := h.contests.Get(c.Request.Context(), tid)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "contest not found"})
			return
		}
		constraints = append(constraints, item.Langs)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": AllowedLanguages(judge.LanguageOptions(), constraints...),
	}})
}

// AllowedLanguages keeps the registry entries accepted by every non-empty
// constraint list, mirroring the upstream intersection semantics.
func AllowedLanguages(base []judge.LanguageOption, constraints ...[]string) []judge.LanguageOption {
	allowed := base
	for _, constraint := range constraints {
		if len(constraint) == 0 {
			continue
		}
		set := make(map[string]bool, len(constraint))
		for _, id := range constraint {
			set[strings.TrimSpace(id)] = true
		}
		filtered := make([]judge.LanguageOption, 0, len(allowed))
		for _, option := range allowed {
			if set[option.ID] {
				filtered = append(filtered, option)
			}
		}
		allowed = filtered
	}
	return allowed
}
