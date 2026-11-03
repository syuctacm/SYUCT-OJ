package handler

import (
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	defaultLimitUserFiles             = 100
	defaultLimitUserFileSize          = 128 << 20
	defaultLimitContestFiles          = 100
	defaultLimitContestFileSize       = 128 << 20
	defaultLimitProblemFiles          = 100
	defaultLimitProblemFileSize       = 256 << 20
	defaultLimitCodeLength            = 128 << 10
	maxConfiguredFileCount            = 10000
	maxConfiguredFileSize       int64 = 10 << 30
)

func configuredLimitInt(c *gin.Context, key string, fallback int) int {
	value := configuredSystemInt64(c, key, int64(fallback), 1, maxConfiguredFileCount)
	return int(value)
}

func configuredLimitBytes(c *gin.Context, key string, fallback int64) int64 {
	return configuredSystemInt64(c, key, fallback, 1, maxConfiguredFileSize)
}

func configuredSystemInt64(c *gin.Context, key string, fallback, min, max int64) int64 {
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
	if err := db.Collection("system").FindOne(c.Request.Context(), bson.M{"_id": key}).Decode(&doc); err != nil {
		return fallback
	}
	value, ok := anyInt64(doc.Value)
	if !ok || value < min || value > max {
		return fallback
	}
	return value
}
