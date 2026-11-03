package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	// RequestDatabaseKey lets list handlers resolve Hydro system settings
	// without adding a database dependency to every handler constructor.
	RequestDatabaseKey = "syuct.request.database"

	defaultPaginationProblem    = 100
	defaultPaginationContest    = 20
	defaultPaginationDiscussion = 50
	defaultPaginationRecord     = 100
	defaultPaginationRanking    = 100
	defaultPaginationSolution   = 20
	defaultPaginationTraining   = 10
	maxPageSize                 = 1000
)

var paginationSettingDefaults = map[string]int{
	settingPaginationProblem:    defaultPaginationProblem,
	settingPaginationContest:    defaultPaginationContest,
	settingPaginationDiscussion: defaultPaginationDiscussion,
	settingPaginationRecord:     defaultPaginationRecord,
	settingPaginationRanking:    defaultPaginationRanking,
	settingPaginationSolution:   defaultPaginationSolution,
	settingPaginationTraining:   defaultPaginationTraining,
}

// parsePaging reads page/limit and applies the matching Hydro `pagination.*`
// default when the caller does not provide an explicit limit.
func parsePaging(c *gin.Context, settingKey string) (int, int) {
	defaultLimit := configuredPaginationDefault(c, settingKey)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	rawLimit := c.Query("limit")
	limit, _ := strconv.Atoi(rawLimit)
	if rawLimit == "" || limit < 1 {
		limit = defaultLimit
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	return page, limit
}

func configuredPaginationDefault(c *gin.Context, settingKey string) int {
	fallback := paginationSettingDefaults[settingKey]
	if fallback < 1 {
		fallback = defaultPaginationContest
	}
	rawDB, ok := c.Get(RequestDatabaseKey)
	if !ok {
		return fallback
	}
	db, ok := rawDB.(*mongo.Database)
	if !ok || db == nil {
		return fallback
	}
	var doc struct {
		Value any `bson:"value"`
	}
	if err := db.Collection("system").FindOne(c.Request.Context(), bson.M{"_id": settingKey}).Decode(&doc); err != nil {
		return fallback
	}
	value, ok := anyInt(doc.Value)
	if !ok || value < 1 {
		return fallback
	}
	if value > maxPageSize {
		return maxPageSize
	}
	return value
}
