package handler

import (
	"crypto/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var checkinLocation = time.FixedZone("CST", 8*60*60)

func (h *UserHandler) checkinInfo(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	today, _ := checkinDates(time.Now())
	result, err := h.users.GetCheckin(c.Request.Context(), uid, today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load checkin"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func (h *UserHandler) checkin(c *gin.Context) {
	uid, ok := h.currentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	today, yesterday := checkinDates(time.Now())
	result, err := h.users.Checkin(c.Request.Context(), uid, today, yesterday, randomLuck())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to check in"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

func checkinDates(now time.Time) (string, string) {
	local := now.In(checkinLocation)
	today := local.Format("2006-01-02")
	yesterday := local.AddDate(0, 0, -1).Format("2006-01-02")
	return today, yesterday
}

func randomLuck() int64 {
	var buffer [1]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return int64(time.Now().UnixNano() % 7)
	}
	return int64(buffer[0] % 7)
}
