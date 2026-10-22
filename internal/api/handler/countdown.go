package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/config"
)

type CountdownItem struct {
	Name string `json:"name"`
	Date string `json:"date"`
	Days int    `json:"days"`
}

type CountdownHandler struct {
	title string
	max   int
	dates []config.CountdownDate
}

func NewCountdownHandler(title string, max int, dates []config.CountdownDate) *CountdownHandler {
	if max <= 0 {
		max = 5
	}
	if title == "" {
		title = "倒计时"
	}
	return &CountdownHandler{title: title, max: max, dates: dates}
}

func (h *CountdownHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/home/countdown", h.get)
}

func (h *CountdownHandler) get(c *gin.Context) {
	now := time.Now().In(checkinLocation)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, checkinLocation)
	items := make([]CountdownItem, 0, h.max)
	for _, entry := range h.dates {
		if len(items) >= h.max {
			break
		}
		target, err := time.ParseInLocation("2006-01-02", entry.Date, checkinLocation)
		if err != nil || target.Before(today) {
			continue
		}
		items = append(items, CountdownItem{
			Name: entry.Name,
			Date: entry.Date,
			Days: int(target.Sub(today).Hours() / 24),
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"title": h.title, "items": items,
	}})
}
