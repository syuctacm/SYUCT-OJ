package handler

import (
	"archive/zip"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/syuctacm/syuct-oj/internal/contest"
)

// exportCodeArchive streams the submissions of a contest (or homework) as a zip
// archive, mirroring the upstream contest_code / homework_code handlers.
func exportCodeArchive(c *gin.Context, repo *contest.Repository, item *contest.Contest, uid int64, isAdmin bool) {
	// Non-owners may only export after the contest ended, like upstream.
	if item.Owner != uid && !isAdmin && !item.EndAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contest has not ended yet"})
		return
	}
	rows, err := repo.ExportRecords(c.Request.Context(), item.DocID, c.Query("all") == "1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load submissions"})
		return
	}
	filename := item.Title + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename)))
	c.Status(http.StatusOK)

	writer := zip.NewWriter(c.Writer)
	defer func() { _ = writer.Close() }()
	for _, row := range rows {
		extension := row.Lang
		if extension == "" {
			extension = "txt"
		}
		entry, err := writer.Create(fmt.Sprintf("U%d_P%d_R%s_S%d@%d.%s",
			row.UID, row.PID, row.RID, row.Status, row.Score, extension))
		if err != nil {
			return
		}
		if _, err := entry.Write([]byte(row.Code)); err != nil {
			return
		}
	}
}
