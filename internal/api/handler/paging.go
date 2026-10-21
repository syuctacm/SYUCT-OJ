package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// defaultPageSize matches Hydro's default `pagination.*` setting.
const defaultPageSize = 20

// parsePaging reads page/limit from the query string, defaulting to page 1 and
// 20 items per page.
func parsePaging(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultPageSize)))
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
