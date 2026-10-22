package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
)

// rejudge resets every submission of a problem and queues it again.
func (h *ProblemHandler) rejudge(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}
	if h.submissions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "submission storage is unavailable"})
		return
	}
	count, err := h.submissions.RejudgeProblem(c.Request.Context(), c.Param("pid"))
	if err != nil {
		if errors.Is(err, problem.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to rejudge problem"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"count": count}})
}

// statistics serves the /p/:pid/statistics page: one page of the problem's
// submissions plus the status/score counters the charts need.
func (h *ProblemHandler) statistics(c *gin.Context) {
	pdoc, err := h.repo.Get(c.Request.Context(), c.Param("pid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}
	if h.submissions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "submission statistics are unavailable"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	direction, _ := strconv.Atoi(c.DefaultQuery("direction", "1"))
	result, err := h.submissions.Statistics(c.Request.Context(), submission.StatisticsFilter{
		DocID:     pdoc.DocID,
		Lang:      strings.TrimSpace(c.Query("lang")),
		Sort:      strings.TrimSpace(c.Query("sort")),
		Direction: direction,
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load submission statistics"})
		return
	}
	ids := make([]int64, 0, len(result.Items))
	seen := map[int64]bool{}
	for _, item := range result.Items {
		if !seen[item.UID] {
			seen[item.UID] = true
			ids = append(ids, item.UID)
		}
	}
	users := []gin.H{}
	if len(ids) > 0 && h.users != nil {
		accounts, err := h.users.FindByIDs(c.Request.Context(), ids)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load submission statistics"})
			return
		}
		for _, account := range accounts {
			users = append(users, gin.H{
				"id": account.ID, "username": account.Uname,
				"displayName": account.DisplayName, "avatar": account.Avatar,
			})
		}
	}
	sortKey := strings.TrimSpace(c.Query("sort"))
	if sortKey == "" {
		sortKey = "time"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"items": result.Items, "total": result.Total, "page": page, "limit": limit,
		"sort": sortKey, "direction": direction, "stats": result.Stats, "users": users,
		"problem": gin.H{"docId": pdoc.DocID, "pid": pdoc.PID, "title": pdoc.Title, "owner": pdoc.Owner},
		"sorts":   submission.StatisticsSorts,
	}})
}
